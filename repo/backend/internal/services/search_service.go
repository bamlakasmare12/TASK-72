package services

import (
	"context"
	"strings"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"
	"wlpr-portal/pkg/pinyin"
)

type SearchService struct {
	searchRepo   *repository.SearchRepository
	taxonomyRepo *repository.TaxonomyRepository
	configSvc    *ConfigService
}

func NewSearchService(searchRepo *repository.SearchRepository, taxonomyRepo *repository.TaxonomyRepository, configSvc *ConfigService) *SearchService {
	return &SearchService{
		searchRepo:   searchRepo,
		taxonomyRepo: taxonomyRepo,
		configSvc:    configSvc,
	}
}

// Search performs full-text search with synonym expansion, pinyin matching, and filtering.
// userID and userRoleIDs are used for feature flag evaluation.
func (s *SearchService) Search(ctx context.Context, req models.SearchRequest, userID int, userRoleIDs []int) (*models.SearchResponse, error) {
	var synonymTagIDs []int
	var appliedSynonyms []string
	var pinyinQuery string

	query := strings.TrimSpace(req.Query)

	if query != "" {
		// Synonym expansion: look up each word in the query
		if s.configSvc.IsFlagEnabled("synonym_search", userID, userRoleIDs) {
			words := strings.Fields(strings.ToLower(query))
			for _, word := range words {
				tagIDs, err := s.taxonomyRepo.FindSynonymMappings(ctx, word)
				if err == nil && len(tagIDs) > 0 {
					synonymTagIDs = append(synonymTagIDs, tagIDs...)
					appliedSynonyms = append(appliedSynonyms, word)
				}
			}
		}

		// Pinyin conversion for Chinese character matching
		if s.configSvc.IsFlagEnabled("pinyin_search", userID, userRoleIDs) {
			pinyinQuery = pinyin.ToPinyin(query)
			// Only use pinyin if it differs from the original query
			if pinyinQuery == strings.ToLower(query) {
				pinyinQuery = ""
			}
		}
	}

	// Deduplicate synonym tag IDs
	synonymTagIDs = dedup(synonymTagIDs)

	resp, err := s.searchRepo.SearchResources(ctx, req, synonymTagIDs, pinyinQuery)
	if err != nil {
		return nil, err
	}

	if len(appliedSynonyms) > 0 {
		resp.Synonyms = appliedSynonyms
	}

	return resp, nil
}

func (s *SearchService) GetResource(ctx context.Context, id int) (*models.Resource, error) {
	return s.searchRepo.GetResourceByID(ctx, id)
}

func (s *SearchService) RecordView(ctx context.Context, userID, resourceID int) error {
	_ = s.searchRepo.IncrementViewCount(ctx, resourceID)
	return s.searchRepo.RecordEvent(ctx, userID, resourceID, "view")
}

func (s *SearchService) GetArchives(ctx context.Context) ([]models.ArchivePage, error) {
	return s.searchRepo.GetArchiveMonthly(ctx)
}

func dedup(ids []int) []int {
	seen := make(map[int]bool)
	var result []int
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}
