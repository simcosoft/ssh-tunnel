package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// TestDialWithRetry_SuccessFirstTry verifies that a successful dial on the first
// attempt returns the value immediately without any retries.
func TestDialWithRetry_SuccessFirstTry(t *testing.T) {
	calls := 0
	got, err := dialWithRetry(3, "test", func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected 'ok', got %q", got)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

// TestDialWithRetry_ExhaustsRetries verifies that when all attempts fail the
// function returns an error. maxRetries=1 avoids the 1s sleep between attempts.
func TestDialWithRetry_ExhaustsRetries(t *testing.T) {
	calls := 0
	_, err := dialWithRetry(1, "test", func() (string, error) {
		calls++
		return "", errors.New("connection refused")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

// TestDialWithRetry_SuccessAfterRetry verifies retry behaviour: the function
// retries on failure and returns the value once dial succeeds.
// NOTE: this test sleeps ~1s due to the retry back-off.
func TestDialWithRetry_SuccessAfterRetry(t *testing.T) {
	calls := 0
	got, err := dialWithRetry(3, "test", func() (string, error) {
		calls++
		if calls < 2 {
			return "", errors.New("temporary error")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected 'ok', got %q", got)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestPasswordCallback(t *testing.T) {
	cb := passwordCallback("s3cr3t")
	pwd, err := cb()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pwd != "s3cr3t" {
		t.Fatalf("expected 's3cr3t', got %q", pwd)
	}
}

func TestPasswordCallback_EmptyPassword(t *testing.T) {
	cb := passwordCallback("")
	pwd, err := cb()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pwd != "" {
		t.Fatalf("expected empty string, got %q", pwd)
	}
}

func TestKeyboardInteractiveChallenge_NoQuestions(t *testing.T) {
	ch := keyboardInteractiveChallenge("pass")
	answers, err := ch("user", "", []string{}, []bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(answers) != 0 {
		t.Fatalf("expected 0 answers, got %d", len(answers))
	}
}

func TestKeyboardInteractiveChallenge_OneQuestion(t *testing.T) {
	ch := keyboardInteractiveChallenge("pass")
	answers, err := ch("user", "", []string{"Password:"}, []bool{false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(answers) != 1 || answers[0] != "pass" {
		t.Fatalf("expected [\"pass\"], got %v", answers)
	}
}

func TestKeyboardInteractiveChallenge_MultipleQuestions(t *testing.T) {
	ch := keyboardInteractiveChallenge("pass")
	_, err := ch("user", "", []string{"Q1:", "Q2:"}, []bool{false, false})
	if err == nil {
		t.Fatal("expected error for multiple questions, got nil")
	}
}

func TestTransferData_CopiesData(t *testing.T) {
	src := bytes.NewBufferString("hello tunnel")
	dst := &bytes.Buffer{}
	quit := make(chan struct{}, 2)

	transferData(dst, src, "test", quit)

	if dst.String() != "hello tunnel" {
		t.Fatalf("expected 'hello tunnel', got %q", dst.String())
	}
	select {
	case <-quit:
		// ok
	default:
		t.Fatal("expected quit signal after transfer")
	}
}

func TestTransferData_SignalsQuitOnError(t *testing.T) {
	r, w := io.Pipe()
	w.CloseWithError(errors.New("simulated read error"))
	quit := make(chan struct{}, 2)

	transferData(io.Discard, r, "test-error", quit)

	select {
	case <-quit:
		// ok — quit must be signalled even on error
	default:
		t.Fatal("expected quit signal on transfer error")
	}
}
