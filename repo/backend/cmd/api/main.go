package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wlpr-portal/internal/handlers"
	"wlpr-portal/internal/middleware"
	"wlpr-portal/internal/repository"
	"wlpr-portal/internal/services"
	appconfig "wlpr-portal/pkg/config"
	"wlpr-portal/pkg/crypto"
	"wlpr-portal/pkg/jwt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load configuration from config/config.js (single source of truth)
	cfg, err := appconfig.Load()
	if err != nil {
		log.Fatalf("Failed to load config.js: %v", err)
	}
	log.Println("Configuration loaded from config.js")

	// Initialize encryption key from config
	if err := crypto.InitEncryptionKeyFromValue(cfg.AESEncryptionKey); err != nil {
		log.Fatalf("Failed to init encryption key: %v", err)
	}

	// Initialize JWT secret from config
	if err := jwt.InitJWTSecretFromValue(cfg.JWTSecret); err != nil {
		log.Fatalf("Failed to init JWT secret: %v", err)
	}

	// Database connection from config
	dbURL := cfg.DatabaseURL
	if dbURL == "" {
		dbURL = "postgres://wlpr:wlpr_secret@localhost:5432/wlpr_portal?sslmode=disable"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Repositories
	userRepo := repository.NewUserRepository(dbPool)
	configRepo := repository.NewConfigRepository(dbPool)
	taxonomyRepo := repository.NewTaxonomyRepository(dbPool)
	searchRepo := repository.NewSearchRepository(dbPool)
	learningRepo := repository.NewLearningRepository(dbPool)
	schedulerRepo := repository.NewSchedulerRepository(dbPool)

	// Services
	configService := services.NewConfigService(configRepo)
	if err := configService.LoadAll(ctx); err != nil {
		log.Fatalf("Failed to load initial configs: %v", err)
	}
	configService.StartBackgroundSync(ctx, 30*time.Second)

	authService := services.NewAuthService(userRepo, configRepo)
	mfaService := services.NewMFAService(userRepo)
	searchService := services.NewSearchService(searchRepo, taxonomyRepo, configService)
	learningService := services.NewLearningService(learningRepo, searchRepo)
	procRepo := repository.NewProcurementRepository(dbPool)
	reconService := services.NewReconciliationService(procRepo, configService)
	exportSinkService := services.NewExportSinkService(configService)

	// Recommendation worker (used by scheduler)
	recWorker := services.NewRecommendationWorker(learningRepo, searchRepo, configService)

	// ================================================================
	// Scheduler: reads job definitions from scheduled_jobs table,
	// executes handlers with retry/compensation, persists run state.
	// ================================================================
	schedulerService := services.NewSchedulerService(schedulerRepo)

	// Register job handlers
	schedulerService.RegisterHandler("recommendation_worker", func(jobCtx context.Context) error {
		log.Println("[scheduler:recommendation_worker] starting nightly rebuild")
		recWorker.RunOnce(jobCtx)
		return nil
	})

	schedulerService.RegisterHandler("session_cleanup", func(jobCtx context.Context) error {
		count, err := userRepo.CleanExpiredSessions(jobCtx)
		if err != nil {
			return fmt.Errorf("session cleanup failed: %w", err)
		}
		log.Printf("[scheduler:session_cleanup] cleaned %d expired sessions", count)
		return nil
	})

	schedulerService.RegisterHandler("archive_refresh", func(jobCtx context.Context) error {
		if _, err := searchRepo.RefreshArchiveViews(jobCtx); err != nil {
			return fmt.Errorf("archive refresh failed: %w", err)
		}
		log.Println("[scheduler:archive_refresh] materialized views refreshed")
		return nil
	})

	// Register compensation handlers
	schedulerService.RegisterCompensation("recommendation_worker", func(jobCtx context.Context, jobName string, lastErr error) error {
		log.Printf("[scheduler:compensation] recommendation_worker failed after retries: %v. Stale recommendations preserved.", lastErr)
		return nil
	})

	schedulerService.RegisterCompensation("session_cleanup", func(jobCtx context.Context, jobName string, lastErr error) error {
		log.Printf("[scheduler:compensation] session_cleanup failed after retries: %v. Sessions will be cleaned on next successful run.", lastErr)
		return nil
	})

	schedulerService.RegisterCompensation("archive_refresh", func(jobCtx context.Context, jobName string, lastErr error) error {
		log.Printf("[scheduler:compensation] archive_refresh failed after retries: %v. Materialized views may be stale.", lastErr)
		return nil
	})

	// Start scheduler (polls every 60 seconds)
	schedulerService.Start(ctx, 60*time.Second)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, mfaService, userRepo)
	configHandler := handlers.NewConfigHandler(configService)
	searchHandler := handlers.NewSearchHandler(searchService)
	taxonomyHandler := handlers.NewTaxonomyHandler(taxonomyRepo)
	learningHandler := handlers.NewLearningHandler(learningService)
	procHandler := handlers.NewProcurementHandler(procRepo, reconService, exportSinkService)

	// Middleware
	authMW := middleware.NewAuthMiddleware(authService, configService)

	// Echo setup
	e := echo.New()
	e.HideBanner = true

	// Global middleware
	e.Use(echomw.Recover())
	e.Use(middleware.RequestLogger())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-App-Version"},
		ExposeHeaders:    []string{"X-App-Deprecated", "X-Min-Version"},
		AllowCredentials: true,
	}))
	e.Use(authMW.AppVersionCheck())

	// Health check
	e.GET("/api/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Public auth routes (no JWT required)
	auth := e.Group("/api/auth")
	auth.POST("/login", authHandler.Login)
	auth.POST("/register", authHandler.Register)
	auth.POST("/mfa/verify", authHandler.VerifyMFA)

	// Protected auth routes
	authProtected := e.Group("/api/auth", authMW.RequireAuth())
	authProtected.POST("/logout", authHandler.Logout)
	authProtected.GET("/me", authHandler.Me)
	authProtected.POST("/mfa/setup", authHandler.SetupMFA)
	authProtected.POST("/mfa/confirm", authHandler.ConfirmMFA)
	authProtected.POST("/mfa/disable", authHandler.DisableMFA)

	// Admin user management (system_admin only)
	adminGroup := e.Group("/api/admin", authMW.RequireAuth(),
		authMW.RequireRole("system_admin"))
	adminGroup.GET("/users", authHandler.ListUsers)
	adminGroup.POST("/users/assign-role", authHandler.AssignRole)

	// Config routes (admin only)
	configGroup := e.Group("/api/config",
		authMW.RequireAuth(),
		authMW.RequireRole("system_admin"),
	)
	configGroup.GET("/all", configHandler.GetAllConfigs)
	configGroup.GET("/:key", configHandler.GetConfig)
	configGroup.PUT("/:key", configHandler.UpdateConfig)
	configGroup.GET("/flags", configHandler.GetAllFlags)
	configGroup.GET("/flags/:key", configHandler.GetFlag)
	configGroup.PUT("/flags/:key", configHandler.UpdateFlag)
	configGroup.GET("/flags/:key/check", configHandler.CheckFlag)

	// Public config check (for feature flag evaluation by authenticated users)
	flagCheck := e.Group("/api/flags", authMW.RequireAuth())
	flagCheck.GET("/:key/check", configHandler.CheckFlag)

	// ================================================================
	// MODULE 2: Catalog & Taxonomy
	// Learner: Read (search/browse). ContentMod: CRUD. Admin: CRUD.
	// Finance/Procurement: No access.
	// ================================================================
	catalogRead := e.Group("/api", authMW.RequireAuth(),
		authMW.RequireRole("learner", "content_moderator", "system_admin"))
	catalogRead.GET("/search", searchHandler.Search)
	catalogRead.GET("/resources/:id", searchHandler.GetResource)
	catalogRead.GET("/archives", searchHandler.GetArchives)

	taxRead := e.Group("/api/taxonomy", authMW.RequireAuth(),
		authMW.RequireRole("learner", "content_moderator", "system_admin"))
	taxRead.GET("/tags", taxonomyHandler.GetTags)
	taxRead.GET("/tags/hierarchy", taxonomyHandler.GetHierarchy)
	taxRead.GET("/tags/:id", taxonomyHandler.GetTag)
	taxRead.GET("/synonyms/:tag_id", taxonomyHandler.GetSynonyms)

	taxWrite := e.Group("/api/taxonomy", authMW.RequireAuth(),
		authMW.RequireRole("content_moderator", "system_admin"))
	taxWrite.POST("/tags", taxonomyHandler.CreateTag)
	taxWrite.POST("/synonyms", taxonomyHandler.CreateSynonym)
	taxWrite.GET("/review-queue", taxonomyHandler.GetReviewQueue)
	taxWrite.GET("/review-queue/audit", taxonomyHandler.GetReviewQueueAudit)
	taxWrite.POST("/review-queue/approve", taxonomyHandler.ApproveReviewItem)
	taxWrite.POST("/review-queue/reject", taxonomyHandler.RejectReviewItem)

	// ================================================================
	// MODULE 1: Learning & Development
	// Learner: Read (assigned/public), Update (own progress/enroll).
	// ContentMod: CRUD (resources & paths). Admin: CRUD (all).
	// Finance/Procurement: No access.
	// ================================================================
	learnRead := e.Group("/api/learning", authMW.RequireAuth(),
		authMW.RequireRole("learner", "content_moderator", "system_admin"))
	learnRead.GET("/paths", learningHandler.GetPaths)
	learnRead.GET("/paths/:id", learningHandler.GetPath)
	learnRead.GET("/recommendations", learningHandler.GetRecommendations)

	// Learner can enroll/update own progress/export own records
	learnSelf := e.Group("/api/learning", authMW.RequireAuth(),
		authMW.RequireRole("learner", "content_moderator", "system_admin"))
	learnSelf.POST("/enroll", learningHandler.Enroll)
	learnSelf.DELETE("/enroll/:path_id", learningHandler.DropEnrollment)
	learnSelf.GET("/enrollments", learningHandler.GetEnrollments)
	learnSelf.GET("/enrollments/:path_id", learningHandler.GetEnrollmentDetail)
	learnSelf.PUT("/progress", learningHandler.UpdateProgress)
	learnSelf.GET("/progress", learningHandler.GetProgress)
	learnSelf.GET("/export", learningHandler.ExportCSV)

	// ================================================================
	// MODULE 3: Procurement & Disputes
	// ProcSpec: Create/Read/Update orders+reviews. Approver: Read+Approve.
	// ContentMod: Read appeals + arbitrate (hide/show/disclaimer).
	// Finance: Read settlement view. Admin: Full.
	// Learner: No access.
	// ================================================================

	// Procurement read (orders, invoices, vendors, billing rules)
	procRead := e.Group("/api/procurement", authMW.RequireAuth(),
		authMW.RequireRole("procurement_specialist", "approver", "finance_analyst", "system_admin"))
	procRead.GET("/vendors", procHandler.GetVendors)
	procRead.GET("/orders", procHandler.GetOrders)
	procRead.GET("/orders/:id", procHandler.GetOrder)
	procRead.GET("/invoices", procHandler.GetInvoices)
	procRead.GET("/invoices/:id", procHandler.GetInvoice)
	procRead.GET("/billing-rules", procHandler.GetBillingRules)

	// Procurement write: create reviews with ratings/text/images, merchant replies
	procWrite := e.Group("/api/procurement", authMW.RequireAuth(),
		authMW.RequireRole("procurement_specialist", "system_admin"))
	procWrite.GET("/reviews", procHandler.GetReviews)
	procWrite.POST("/reviews", procHandler.CreateReview)
	procWrite.POST("/reviews/reply", procHandler.CreateMerchantReply)

	// Approval actions (approve orders, match invoices)
	procApprove := e.Group("/api/procurement", authMW.RequireAuth(),
		authMW.RequireRole("approver", "system_admin"))
	procApprove.PUT("/orders/:id/approve", procHandler.ApproveOrder)
	procApprove.POST("/invoices/match", procHandler.MatchInvoice)

	// Dispute read (procurement_specialist, content_moderator, approver, admin)
	disputeRead := e.Group("/api/procurement", authMW.RequireAuth(),
		authMW.RequireRole("procurement_specialist", "content_moderator", "approver", "system_admin"))
	disputeRead.GET("/disputes", procHandler.GetDisputes)
	disputeRead.GET("/disputes/:id", procHandler.GetDispute)

	// Dispute creation (procurement users create appeals with evidence)
	disputeCreate := e.Group("/api/procurement", authMW.RequireAuth(),
		authMW.RequireRole("procurement_specialist", "system_admin"))
	disputeCreate.POST("/disputes", procHandler.CreateDispute)

	// Dispute arbitration transitions (content_moderator + admin)
	disputeArbitrate := e.Group("/api/procurement", authMW.RequireAuth(),
		authMW.RequireRole("content_moderator", "system_admin"))
	disputeArbitrate.POST("/disputes/transition", procHandler.TransitionDispute)

	// ================================================================
	// MODULE 4: Reconciliation & Finance
	// Finance: Create AR/AP, Read all, Update settlement status.
	// Approver: Read variances, Approve write-offs.
	// ProcSpec: Read relevant order costs only.
	// Learner/ContentMod: No access.
	// ================================================================

	// Finance: read ledger, create AR/AP entries, create settlements, compare, export
	finRead := e.Group("/api/procurement", authMW.RequireAuth(),
		authMW.RequireRole("finance_analyst", "system_admin"))
	finRead.GET("/ledger", procHandler.GetLedger)
	finRead.POST("/ledger", procHandler.CreateLedgerEntry)
	finRead.GET("/cost-allocation", procHandler.GetCostAllocation)
	finRead.POST("/reconciliation/compare", procHandler.CompareStatement)
	finRead.POST("/settlements", procHandler.CreateSettlement)
	finRead.GET("/export/ledger", procHandler.ExportLedger)
	finRead.GET("/export/settlements", procHandler.ExportSettlements)

	// Settlements: finance reads + manages, approver reads + approves
	settleGroup := e.Group("/api/procurement", authMW.RequireAuth(),
		authMW.RequireRole("finance_analyst", "approver", "system_admin"))
	settleGroup.GET("/settlements", procHandler.GetSettlements)
	settleGroup.POST("/settlements/transition", procHandler.TransitionSettlement)

	// Start server
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Printf("Starting server on %s", addr)
		if err := e.Start(addr); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}
	cancel()
	log.Println("Server exited")
}
