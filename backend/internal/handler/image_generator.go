package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const webImageGeneratorAPIKeyName = "Web Image Generator"

var (
	webImageAllowedModels = map[string]struct{}{
		"gpt-image-1.5": {},
		"gpt-image-1-mini": {},
	}
	webImageAllowedSizes = map[string]string{
		"1024x1024": "1K",
		"1536x1024": "2K",
		"1024x1536": "2K",
		"2048x1152": "2K",
		"1152x2048": "2K",
	}
	webImageAllowedQualities = map[string]struct{}{
		"auto": {},
		"low": {},
		"medium": {},
		"high": {},
	}
)

type webImageGenerateRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Size    string `json:"size"`
	Quality string `json:"quality"`
	N       int    `json:"n"`
}

func (h *OpenAIGatewayHandler) GenerateWebImage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthorized")
		return
	}
	if h.apiKeyService == nil || h.gatewayService == nil || h.billingCacheService == nil {
		response.InternalError(c, "Image generation is not available")
		return
	}

	var req webImageGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	normalized, sizeTier, err := normalizeWebImageRequest(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	apiKey, subscription, err := h.prepareWebImageAPIKey(c.Request.Context(), subject.UserID, normalized, sizeTier)
	if err != nil {
		writeWebImageError(c, err)
		return
	}
	if apiKey.User != nil && apiKey.User.Concurrency > 0 {
		subject.Concurrency = apiKey.User.Concurrency
		c.Set(string(middleware2.ContextKeyUser), subject)
	}

	body, err := json.Marshal(map[string]any{
		"model":           normalized.Model,
		"prompt":          normalized.Prompt,
		"size":            normalized.Size,
		"quality":         normalized.Quality,
		"n":               normalized.N,
		"response_format": "b64_json",
	})
	if err != nil {
		response.InternalError(c, "Failed to build image request")
		return
	}

	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	if subscription != nil {
		c.Set(string(middleware2.ContextKeySubscription), subscription)
	}
	c.Request.URL.Path = "/v1/images/generations"
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Images(c)
}

func normalizeWebImageRequest(req webImageGenerateRequest) (webImageGenerateRequest, string, error) {
	req.Model = strings.TrimSpace(req.Model)
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Size = strings.TrimSpace(req.Size)
	req.Quality = strings.ToLower(strings.TrimSpace(req.Quality))
	if req.Model == "" {
		req.Model = "gpt-image-1.5"
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.Quality == "" {
		req.Quality = "auto"
	}
	if req.N == 0 {
		req.N = 1
	}
	if _, ok := webImageAllowedModels[req.Model]; !ok {
		return req, "", fmt.Errorf("unsupported image model")
	}
	if req.Prompt == "" {
		return req, "", fmt.Errorf("prompt is required")
	}
	if len([]rune(req.Prompt)) > 4000 {
		return req, "", fmt.Errorf("prompt is too long")
	}
	sizeTier, ok := webImageAllowedSizes[req.Size]
	if !ok {
		return req, "", fmt.Errorf("unsupported image size")
	}
	if _, ok := webImageAllowedQualities[req.Quality]; !ok {
		return req, "", fmt.Errorf("unsupported image quality")
	}
	if req.N < 1 || req.N > 4 {
		return req, "", fmt.Errorf("image count must be between 1 and 4")
	}
	return req, sizeTier, nil
}

func (h *OpenAIGatewayHandler) prepareWebImageAPIKey(ctx context.Context, userID int64, req webImageGenerateRequest, sizeTier string) (*service.APIKey, *service.UserSubscription, error) {
	groups, err := h.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	group, subscription, err := h.selectWebImageGroup(ctx, userID, groups)
	if err != nil {
		return nil, nil, err
	}

	apiKey, err := h.findOrCreateWebImageAPIKey(ctx, userID, group.ID)
	if err != nil {
		return nil, nil, err
	}
	if apiKey.User == nil || apiKey.Group == nil {
		apiKey, err = h.apiKeyService.GetByID(ctx, apiKey.ID)
		if err != nil {
			return nil, nil, err
		}
	}
	if apiKey.User == nil || apiKey.Group == nil {
		return nil, nil, fmt.Errorf("image API key is incomplete")
	}

	if err := h.checkWebImageEstimatedCost(ctx, apiKey.User, apiKey, apiKey.Group, subscription, req.Model, sizeTier, req.N); err != nil {
		return nil, nil, err
	}
	return apiKey, subscription, nil
}

func (h *OpenAIGatewayHandler) selectWebImageGroup(ctx context.Context, userID int64, groups []service.Group) (*service.Group, *service.UserSubscription, error) {
	for i := range groups {
		group := &groups[i]
		if group.Platform != service.PlatformOpenAI || !group.IsActive() || !service.GroupAllowsImageGeneration(group) {
			continue
		}
		var subscription *service.UserSubscription
		if group.IsSubscriptionType() {
			if h.subscriptionService == nil {
				continue
			}
			sub, err := h.subscriptionService.GetActiveSubscription(ctx, userID, group.ID)
			if err != nil {
				continue
			}
			subscription = sub
		}
		return group, subscription, nil
	}
	return nil, nil, fmt.Errorf("image generation is not enabled for your group")
}

func (h *OpenAIGatewayHandler) findOrCreateWebImageAPIKey(ctx context.Context, userID, groupID int64) (*service.APIKey, error) {
	keys, _, err := h.apiKeyService.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 1000}, service.APIKeyListFilters{
		Status:  service.StatusAPIKeyActive,
		GroupID: &groupID,
	})
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if key.IsActive() && key.GroupID != nil && *key.GroupID == groupID && !key.IsExpired() && !key.IsQuotaExhausted() {
			return h.apiKeyService.GetByID(ctx, key.ID)
		}
	}
	created, err := h.apiKeyService.Create(ctx, userID, service.CreateAPIKeyRequest{
		Name:    webImageGeneratorAPIKeyName,
		GroupID: &groupID,
	})
	if err != nil {
		return nil, err
	}
	return h.apiKeyService.GetByID(ctx, created.ID)
}

func (h *OpenAIGatewayHandler) checkWebImageEstimatedCost(ctx context.Context, user *service.User, apiKey *service.APIKey, group *service.Group, subscription *service.UserSubscription, model string, sizeTier string, count int) error {
	if err := h.billingCacheService.CheckBillingEligibility(ctx, user, apiKey, group, subscription); err != nil {
		return err
	}
	if group != nil && group.IsSubscriptionType() && subscription != nil {
		return nil
	}
	multiplier := 1.0
	if group != nil {
		multiplier = group.RateMultiplier
		if user != nil && user.GroupRates != nil {
			if rate, ok := user.GroupRates[group.ID]; ok {
				multiplier = rate
			}
		}
		if group.ImageRateIndependent {
			multiplier = group.ImageRateMultiplier
		}
	}
	if multiplier < 0 {
		multiplier = 0
	}
	var groupConfig *service.ImagePriceConfig
	if group != nil {
		groupConfig = &service.ImagePriceConfig{
			Price1K: group.ImagePrice1K,
			Price2K: group.ImagePrice2K,
			Price4K: group.ImagePrice4K,
		}
	}
	cost := service.NewBillingService(nil, nil).CalculateImageCost(model, sizeTier, count, groupConfig, multiplier)
	if cost != nil && user != nil {
		balance := user.Balance
		if h.billingCacheService != nil {
			if cachedBalance, err := h.billingCacheService.GetUserBalance(ctx, user.ID); err == nil {
				balance = cachedBalance
			}
		}
		if balance < cost.ActualCost {
			return fmt.Errorf("insufficient balance: estimated cost is %.6f", cost.ActualCost)
		}
	}
	return nil
}

func writeWebImageError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	lowerMsg := strings.ToLower(msg)
	switch {
	case strings.Contains(lowerMsg, "insufficient balance"):
		response.Forbidden(c, msg)
	case strings.Contains(lowerMsg, "not enabled"):
		response.Forbidden(c, msg)
	case strings.Contains(lowerMsg, "billing") || strings.Contains(lowerMsg, "quota") || strings.Contains(lowerMsg, "rate"):
		response.Forbidden(c, msg)
	default:
		response.InternalError(c, "Failed to prepare image generation")
	}
}
