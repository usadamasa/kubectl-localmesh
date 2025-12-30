package hosts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateHostsFile_Valid_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	// 空ファイルを作成
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	state, err := validateHostsFile()
	if err != nil {
		t.Fatalf("validateHostsFile failed: %v", err)
	}

	if !state.isValid {
		t.Errorf("expected isValid=true, got false")
	}
	if state.markerBlockCount != 0 {
		t.Errorf("expected markerBlockCount=0, got %d", state.markerBlockCount)
	}
	if len(state.problems) != 0 {
		t.Errorf("expected no problems, got %v", state.problems)
	}
}

// TestValidateHostsFile_Valid_NoMarkers は、マーカーがないファイルが有効と判定されることをテストする
func TestValidateHostsFile_Valid_NoMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	content := "127.0.0.1 localhost\n::1 localhost\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	state, err := validateHostsFile()
	if err != nil {
		t.Fatalf("validateHostsFile failed: %v", err)
	}

	if !state.isValid {
		t.Errorf("expected isValid=true, got false")
	}
	if state.markerBlockCount != 0 {
		t.Errorf("expected markerBlockCount=0, got %d", state.markerBlockCount)
	}
	if len(state.problems) != 0 {
		t.Errorf("expected no problems, got %v", state.problems)
	}
}

// TestValidateHostsFile_Invalid_SingleBlock は、正常なマーカーブロック1つが無効と判定されることをテストする
func TestValidateHostsFile_Invalid_SingleBlock(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	content := "127.0.0.1 localhost\n\n" +
		markerStart + "\n" +
		"127.0.0.1 test.localhost\n" +
		markerEnd + "\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	state, err := validateHostsFile()
	if err != nil {
		t.Fatalf("validateHostsFile failed: %v", err)
	}

	if state.isValid {
		t.Errorf("expected isValid=false, got true")
	}
	if state.markerBlockCount != 1 {
		t.Errorf("expected markerBlockCount=1, got %d", state.markerBlockCount)
	}
	if len(state.problems) == 0 {
		t.Errorf("expected problems, got none")
	}
	// 問題メッセージに "Existing kubectl-localmesh entries found" が含まれることを確認
	if !strings.Contains(state.problems[0], "Existing kubectl-localmesh entries found") {
		t.Errorf("expected problem about existing entries, got %s", state.problems[0])
	}
}

// TestValidateHostsFile_Invalid_MultipleBlocks は、複数のマーカーブロックが無効と判定されることをテストする
func TestValidateHostsFile_Invalid_MultipleBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	content := "127.0.0.1 localhost\n\n" +
		markerStart + "\n" +
		"127.0.0.1 old.localhost\n" +
		markerEnd + "\n\n" +
		markerStart + "\n" +
		"127.0.0.1 test.localhost\n" +
		markerEnd + "\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	state, err := validateHostsFile()
	if err != nil {
		t.Fatalf("validateHostsFile failed: %v", err)
	}

	if state.isValid {
		t.Errorf("expected isValid=false, got true")
	}
	if state.markerBlockCount != 2 {
		t.Errorf("expected markerBlockCount=2, got %d", state.markerBlockCount)
	}
	if len(state.problems) == 0 {
		t.Errorf("expected problems, got none")
	}
	// 問題メッセージに "Multiple marker blocks found" が含まれることを確認
	if !strings.Contains(state.problems[0], "Multiple marker blocks found") {
		t.Errorf("expected problem about multiple blocks, got %s", state.problems[0])
	}
}

// TestValidateHostsFile_Invalid_UnclosedBlock は、開始マーカーのみが無効と判定されることをテストする
func TestValidateHostsFile_Invalid_UnclosedBlock(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	content := "127.0.0.1 localhost\n\n" +
		markerStart + "\n" +
		"127.0.0.1 test.localhost\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	state, err := validateHostsFile()
	if err != nil {
		t.Fatalf("validateHostsFile failed: %v", err)
	}

	if state.isValid {
		t.Errorf("expected isValid=false, got true")
	}
	if !state.hasUnclosedBlock {
		t.Errorf("expected hasUnclosedBlock=true, got false")
	}
	if len(state.problems) == 0 {
		t.Errorf("expected problems, got none")
	}
	// 問題メッセージに "Unclosed block" が含まれることを確認
	foundUnclosedProblem := false
	for _, p := range state.problems {
		if strings.Contains(p, "Unclosed block") {
			foundUnclosedProblem = true
			break
		}
	}
	if !foundUnclosedProblem {
		t.Errorf("expected problem about unclosed block, got %v", state.problems)
	}
}

// TestValidateHostsFile_Invalid_OrphanEnd は、終了マーカーのみが無効と判定されることをテストする
func TestValidateHostsFile_Invalid_OrphanEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	content := "127.0.0.1 localhost\n" +
		markerEnd + "\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	state, err := validateHostsFile()
	if err != nil {
		t.Fatalf("validateHostsFile failed: %v", err)
	}

	if state.isValid {
		t.Errorf("expected isValid=false, got true")
	}
	if !state.hasOrphanEnd {
		t.Errorf("expected hasOrphanEnd=true, got false")
	}
	if len(state.problems) == 0 {
		t.Errorf("expected problems, got none")
	}
	// 問題メッセージに "End marker without start marker" が含まれることを確認
	if !strings.Contains(state.problems[0], "End marker without start marker") {
		t.Errorf("expected problem about orphan end, got %s", state.problems[0])
	}
}

// TestValidateHostsFile_Invalid_NestedMarkers は、ネストしたマーカーが無効と判定されることをテストする
func TestValidateHostsFile_Invalid_NestedMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	content := markerStart + "\n" +
		markerStart + "\n" +
		"127.0.0.1 test.localhost\n" +
		markerEnd + "\n" +
		markerEnd + "\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	state, err := validateHostsFile()
	if err != nil {
		t.Fatalf("validateHostsFile failed: %v", err)
	}

	if state.isValid {
		t.Errorf("expected isValid=false, got true")
	}
	if !state.hasNestedMarkers {
		t.Errorf("expected hasNestedMarkers=true, got false")
	}
	if len(state.problems) == 0 {
		t.Errorf("expected problems, got none")
	}
	// 問題メッセージに "Nested start marker" が含まれることを確認
	foundNestedProblem := false
	for _, p := range state.problems {
		if strings.Contains(p, "Nested start marker") {
			foundNestedProblem = true
			break
		}
	}
	if !foundNestedProblem {
		t.Errorf("expected problem about nested markers, got %v", state.problems)
	}
}

// TestValidateHostsFile_Invalid_Combined は、複合的な問題が無効と判定されることをテストする
func TestValidateHostsFile_Invalid_Combined(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	// 複数のブロック + 未完結ブロック
	content := markerStart + "\n" +
		"127.0.0.1 test1.localhost\n" +
		markerEnd + "\n" +
		markerStart + "\n" +
		"127.0.0.1 test2.localhost\n" // 終了マーカーがない
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	state, err := validateHostsFile()
	if err != nil {
		t.Fatalf("validateHostsFile failed: %v", err)
	}

	if state.isValid {
		t.Errorf("expected isValid=false, got true")
	}
	if state.markerBlockCount != 2 {
		t.Errorf("expected markerBlockCount=2, got %d", state.markerBlockCount)
	}
	if !state.hasUnclosedBlock {
		t.Errorf("expected hasUnclosedBlock=true, got false")
	}
	// 複数の問題が報告されることを確認
	if len(state.problems) < 2 {
		t.Errorf("expected at least 2 problems, got %d", len(state.problems))
	}
}

// TestAddEntries_RejectsInvalidState は、AddEntries()が無効状態を拒否することをテストする
func TestAddEntries_RejectsInvalidState(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	// 既存のマーカーブロックを作成
	content := "127.0.0.1 localhost\n\n" +
		markerStart + "\n" +
		"127.0.0.1 old.localhost\n" +
		markerEnd + "\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hostnames := []string{"new.localhost"}

	// AddEntries()はエラーを返すべき
	err := AddEntries(hostnames)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// エラーが hostsFileCorruptedError であることを確認
	var corruptedErr *hostsFileCorruptedError
	if !errors.As(err, &corruptedErr) {
		t.Errorf("expected hostsFileCorruptedError, got %T", err)
	}
}

// TestHostsFileCorruptedError_Message は、hostsFileCorruptedErrorのエラーメッセージをテストする
func TestHostsFileCorruptedError_Message(t *testing.T) {
	state := &hostsFileState{
		isValid:          false,
		markerBlockCount: 1,
		problems: []string{
			"Existing kubectl-localmesh entries found (1 block). Clean shutdown may have failed.",
		},
		fileContent: "127.0.0.1 localhost\n\n" +
			markerStart + "\n" +
			"127.0.0.1 test.localhost\n" +
			markerEnd + "\n",
	}

	err := newHostsFileCorruptedError(state)
	errMsg := err.Error()

	// エラーメッセージに必要な情報が含まれることを確認
	requiredStrings := []string{
		"/etc/hosts is in an invalid state",
		"Existing kubectl-localmesh entries found",
		"Current /etc/hosts content:",
		"To fix:",
		markerStart,
		markerEnd,
	}

	for _, required := range requiredStrings {
		if !strings.Contains(errMsg, required) {
			t.Errorf("error message missing required string %q", required)
		}
	}
}
