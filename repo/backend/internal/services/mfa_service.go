package services

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"wlpr-portal/internal/repository"
	"wlpr-portal/pkg/crypto"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type MFAService struct {
	userRepo *repository.UserRepository
}

func NewMFAService(userRepo *repository.UserRepository) *MFAService {
	return &MFAService{userRepo: userRepo}
}

// SetupMFA generates a new TOTP secret, encrypts it, and stores it for the user.
// Returns the secret (for QR code) and provisioning URI.
func (s *MFAService) SetupMFA(ctx context.Context, userID int, username string) (secret string, provisioningURI string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "WLPR-Portal",
		AccountName: username,
		Period:      30,
		SecretSize:  32,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	encSecret, err := crypto.EncryptString(key.Secret())
	if err != nil {
		return "", "", fmt.Errorf("failed to encrypt MFA secret: %w", err)
	}

	if err := s.userRepo.SetMFASecret(ctx, userID, encSecret); err != nil {
		return "", "", fmt.Errorf("failed to store MFA secret: %w", err)
	}

	return key.Secret(), key.URL(), nil
}

// ConfirmMFA validates the initial TOTP code and enables MFA, generating recovery codes.
func (s *MFAService) ConfirmMFA(ctx context.Context, userID int, code string) ([]string, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	if user.MFASecretEnc == nil {
		return nil, fmt.Errorf("MFA setup not initiated")
	}

	secret, err := crypto.DecryptString(user.MFASecretEnc)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt MFA secret: %w", err)
	}

	if !ValidateTOTP(secret, code) {
		return nil, fmt.Errorf("invalid TOTP code")
	}

	recoveryCodes := generateRecoveryCodes(8)

	recoveryJSON, _ := json.Marshal(recoveryCodes)
	encRecovery, err := crypto.Encrypt(recoveryJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt recovery codes: %w", err)
	}

	if err := s.userRepo.EnableMFA(ctx, userID, encRecovery); err != nil {
		return nil, fmt.Errorf("failed to enable MFA: %w", err)
	}

	return recoveryCodes, nil
}

// DisableMFA removes MFA from a user account.
func (s *MFAService) DisableMFA(ctx context.Context, userID int) error {
	return s.userRepo.DisableMFA(ctx, userID)
}

// ValidateTOTP checks a TOTP code against a secret, allowing +-1 period skew.
func ValidateTOTP(secret, code string) bool {
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:     1,
		Digits:   otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && valid
}

func generateRecoveryCodes(count int) []string {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		b := make([]byte, 5)
		_, _ = rand.Read(b)
		raw := strings.ToUpper(base32.StdEncoding.EncodeToString(b))
		if len(raw) > 8 {
			raw = raw[:8]
		}
		codes[i] = fmt.Sprintf("%s-%s", raw[:4], raw[4:])
	}
	return codes
}
