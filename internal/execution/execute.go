package execution

import (
	"context"
	"errors"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

func executeTarget(ctx context.Context, target provider.Provider, capability, model string, request Request) (Response, error) {
	switch capability {
	case config.CapabilityText, config.CapabilityStructured:
		return executeChat(ctx, target, model, request)
	case config.CapabilityVision, config.CapabilityOCR:
		return executeVision(ctx, target, model, request)
	case config.CapabilityEmbedding:
		return executeEmbedding(ctx, target, model, request)
	case config.CapabilityRerank:
		return executeRerank(ctx, target, model, request)
	default:
		return Response{}, ErrCapability
	}
}

func executeChat(ctx context.Context, target provider.Provider, model string, request Request) (Response, error) {
	messages := request.Messages
	if len(messages) == 0 {
		messages = []provider.Message{{Role: "user", Content: request.Prompt}}
	}
	result, err := target.Chat(ctx, provider.ChatRequest{
		Model: model, Messages: messages, Temperature: request.Temperature, MaxTokens: request.MaxTokens,
	})
	if err != nil {
		return Response{}, err
	}
	return Response{Content: result.Content, Usage: result.Usage}, nil
}

func executeVision(ctx context.Context, target provider.Provider, model string, request Request) (Response, error) {
	images, err := decodeImages(request.Images)
	if err != nil {
		return Response{}, err
	}
	result, err := target.Vision(ctx, provider.VisionRequest{
		Model: model, Prompt: request.Prompt, Images: images,
		Temperature: request.Temperature, MaxTokens: request.MaxTokens,
	})
	if err != nil {
		return Response{}, err
	}
	return Response{Content: result.Content, Usage: result.Usage}, nil
}

func executeEmbedding(ctx context.Context, target provider.Provider, model string, request Request) (Response, error) {
	embedder, ok := target.(provider.EmbeddingProvider)
	if !ok {
		return Response{}, ErrEmbeddingSupport
	}
	result, err := embedder.Embeddings(ctx, provider.EmbeddingRequest{Model: model, Inputs: request.Inputs})
	if err != nil {
		return Response{}, err
	}
	return Response{Vectors: result.Vectors}, nil
}

func executeRerank(ctx context.Context, target provider.Provider, model string, request Request) (Response, error) {
	reranker, ok := target.(provider.RerankProvider)
	if !ok {
		return Response{}, ErrRerankingSupport
	}
	if request.Query == "" || len(request.Documents) == 0 {
		return Response{}, errors.New("reranking requires a query and documents")
	}
	result, err := reranker.Rerank(ctx, provider.RerankRequest{Model: model, Query: request.Query, Documents: request.Documents})
	if err != nil {
		return Response{}, err
	}
	return Response{RerankResults: result.Results}, nil
}
