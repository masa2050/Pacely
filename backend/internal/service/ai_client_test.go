package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const validGeminiResponse = `{"candidates":[{"content":{"parts":[{"text":"{\"advice_text\":\"テスト\",\"next_menu\":{\"menu_type\":\"ジョグ\",\"distance_km\":5,\"pace_sec_per_km\":360,\"note\":\"\"}}"}]}}]}`

// modelごとに返すstatusを指定できる偽のGemini。呼ばれたモデル名を順に記録する。
func fakeGemini(t *testing.T, statuses map[string]int) (*GeminiClient, *[]string) {
	t.Helper()
	var mu sync.Mutex
	called := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		called = append(called, name)
		mu.Unlock()
		status := statuses[name]
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(validGeminiResponse))
			return
		}
		_, _ = w.Write([]byte(`{"error":{"status":"UNAVAILABLE"}}`))
	}))
	t.Cleanup(srv.Close)
	return &GeminiClient{apiKey: "test", httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: srv.URL}, &called
}

func TestGenerateAdviceFallsBackWhenPrimaryBusy(t *testing.T) {
	for _, busyStatus := range []int{http.StatusServiceUnavailable, http.StatusTooManyRequests} {
		c, called := fakeGemini(t, map[string]int{geminiModels[0]: busyStatus, geminiModels[1]: http.StatusOK})
		got, err := c.GenerateAdvice(context.Background(), AdvicePromptInput{})
		if err != nil {
			t.Fatalf("status %d: 予備モデルで成功するはずが %v", busyStatus, err)
		}
		if got.AdviceText != "テスト" {
			t.Errorf("AdviceText = %q", got.AdviceText)
		}
		if len(*called) != 2 || (*called)[1] != geminiModels[1] {
			t.Errorf("呼び出し順 = %v", *called)
		}
	}
}

func TestGenerateAdviceUsesPrimaryWhenAvailable(t *testing.T) {
	c, called := fakeGemini(t, map[string]int{geminiModels[0]: http.StatusOK})
	if _, err := c.GenerateAdvice(context.Background(), AdvicePromptInput{}); err != nil {
		t.Fatal(err)
	}
	if len(*called) != 1 {
		t.Errorf("メインが成功したら予備は呼ばない: %v", *called)
	}
}

func TestGenerateAdviceFailsWhenAllModelsBusy(t *testing.T) {
	c, called := fakeGemini(t, map[string]int{geminiModels[0]: 503, geminiModels[1]: 503})
	_, err := c.GenerateAdvice(context.Background(), AdvicePromptInput{})
	if !errors.Is(err, ErrExternalAPI) {
		t.Errorf("ErrExternalAPIを期待: %v", err)
	}
	if len(*called) != len(geminiModels) {
		t.Errorf("全モデルを試すはず: %v", *called)
	}
}

// 400や認証エラーは別モデルでも直らないため、切り替えずに失敗させる。
func TestGenerateAdviceDoesNotFallBackOnClientError(t *testing.T) {
	c, called := fakeGemini(t, map[string]int{geminiModels[0]: http.StatusBadRequest, geminiModels[1]: http.StatusOK})
	_, err := c.GenerateAdvice(context.Background(), AdvicePromptInput{})
	if !errors.Is(err, ErrExternalAPI) {
		t.Errorf("ErrExternalAPIを期待: %v", err)
	}
	if len(*called) != 1 {
		t.Errorf("予備モデルを呼んではいけない: %v", *called)
	}
}
