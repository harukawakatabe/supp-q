package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEvidencePipelinePersistsOCRBeforeStructuring(t *testing.T) {
	var persisted atomic.Bool
	var structureCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/chat/completions":
			_ = json.NewEncoder(response).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "Brand: Evidence Labs\nProduct: D3\n60 softgels"}}}})
		case "/v1/messages":
			structureCalls.Add(1)
			if !persisted.Load() {
				t.Error("structuring was called before OCR evidence was persisted")
			}
			_ = json.NewEncoder(response).Encode(map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"status":"recognized","confidence":0.91,"fields":{"brand":"Evidence Labs","productName":"D3","count":60,"unit":"softgels"}}`}}})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	pipeline := NewEvidencePipeline(PipelineConfig{
		Mode:      "ocr_llm",
		OCR:       TextOCRConfig{BaseURL: server.URL, APIKey: "ocr-secret", Model: "ocr-model"},
		Structure: AnthropicConfig{BaseURL: server.URL, APIKey: "kimi-secret", Model: "kimi-model", AuthMode: "x-api-key", DisableThinking: true},
	})
	result, err := pipeline.Recognize(context.Background(), "front", "image/jpeg", []byte("image"), func(_ context.Context, evidence Evidence) error {
		if evidence.RawText != "Brand: Evidence Labs\nProduct: D3\n60 softgels" || evidence.Model != "ocr-model" {
			t.Fatalf("unexpected evidence: %+v", evidence)
		}
		persisted.Store(true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if structureCalls.Load() != 1 || result.Trace.SelectedRoute != "ocr_llm" {
		t.Fatalf("unexpected pipeline result: %+v", result)
	}
	if result.Candidate.RawText == "" || result.Candidate.Fields["productName"] != "D3" {
		t.Fatalf("OCR evidence or structured fields missing: %+v", result.Candidate)
	}
}

func TestEvidencePipelineStopsWhenEvidenceCannotBePersisted(t *testing.T) {
	var structureCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/v1/messages" {
			structureCalls.Add(1)
		}
		_ = json.NewEncoder(response).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "visible OCR text"}}}})
	}))
	defer server.Close()
	pipeline := NewEvidencePipeline(PipelineConfig{Mode: "ocr_llm", OCR: TextOCRConfig{BaseURL: server.URL, APIKey: "secret", Model: "ocr"}, Structure: AnthropicConfig{BaseURL: server.URL, APIKey: "secret", Model: "kimi"}})
	_, err := pipeline.Recognize(context.Background(), "front", "image/jpeg", []byte("image"), func(context.Context, Evidence) error {
		return context.Canceled
	})
	if err == nil || structureCalls.Load() != 0 {
		t.Fatalf("persistence failure must stop structuring: err=%v calls=%d", err, structureCalls.Load())
	}
}

func TestUsableOCRTextRejectsProviderGarbage(t *testing.T) {
	for _, value := range []string{"", "}", "} { { { { { { {", "***"} {
		if usableOCRText(value) {
			t.Fatalf("provider garbage accepted as OCR evidence: %q", value)
		}
	}
	for _, value := range []string{"Vitamin C 500 mg", "维生素 C 500 毫克", "EXP 2027-03-31"} {
		if !usableOCRText(value) {
			t.Fatalf("valid OCR evidence rejected: %q", value)
		}
	}
}

func TestFrontStructurePromptDoesNotTreatStrengthAsDose(t *testing.T) {
	value := structurePrompt("front", "VITAMIN C 500 mg")
	if !strings.Contains(value, "500 mg is NOT dose") {
		t.Fatal("front-label strength guard is missing")
	}
}

func TestDirectVisionPersistsReturnedVisibleText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"status":"recognized","confidence":0.9,"rawText":"Vitamin C 500 mg","fields":{"productName":"Vitamin C"}}`}}}})
	}))
	defer server.Close()
	vision := NewOpenAIVision(VisionConfig{BaseURL: server.URL, APIKey: "secret", Model: "vl-model"})
	var saved Evidence
	result, err := vision.Recognize(context.Background(), "front", "image/png", []byte("image"), func(_ context.Context, evidence Evidence) error {
		saved = evidence
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.RawText != "Vitamin C 500 mg" || saved.Model != "vl-model" || result.Trace.SelectedRoute != "direct_vl" {
		t.Fatalf("direct visible text was not persisted: evidence=%+v result=%+v", saved, result)
	}
}
