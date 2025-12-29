package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setTestHostsFile は、テスト用の一時ファイルパスを設定し、テスト終了時に元に戻す
func setTestHostsFile(t *testing.T, path string) {
	t.Helper()
	original := hostsFile
	hostsFile = path
	t.Cleanup(func() {
		hostsFile = original
	})
}

// TestAddEntries_EmptyFile は、空ファイルへの初回追加をテストする
func TestAddEntries_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	// 空ファイルを作成
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hostnames := []string{"test.localhost", "api.localhost"}

	if err := AddEntries(hostnames); err != nil {
		t.Fatalf("AddEntries failed: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	// 期待される内容:
	// # kubectl-local-mesh: managed by kubectl-local-mesh
	// 127.0.0.1 test.localhost
	// 127.0.0.1 api.localhost
	// # kubectl-local-mesh: end
	// (最後に1つの改行)

	expected := markerStart + "\n" +
		"127.0.0.1 test.localhost\n" +
		"127.0.0.1 api.localhost\n" +
		markerEnd + "\n"

	if string(content) != expected {
		t.Errorf("unexpected content:\ngot:\n%q\nwant:\n%q", string(content), expected)
	}
}

// TestAddEntries_ExistingContent は、既存内容がある場合のテスト
func TestAddEntries_ExistingContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	// 既存内容を書き込み
	initialContent := "127.0.0.1 localhost\n::1 localhost\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hostnames := []string{"test.localhost"}

	if err := AddEntries(hostnames); err != nil {
		t.Fatalf("AddEntries failed: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	// 期待される内容:
	// 127.0.0.1 localhost
	// ::1 localhost
	// (空行)
	// # kubectl-local-mesh: managed by kubectl-local-mesh
	// 127.0.0.1 test.localhost
	// # kubectl-local-mesh: end
	// (最後に1つの改行)

	expected := "127.0.0.1 localhost\n" +
		"::1 localhost\n" +
		"\n" +
		markerStart + "\n" +
		"127.0.0.1 test.localhost\n" +
		markerEnd + "\n"

	if string(content) != expected {
		t.Errorf("unexpected content:\ngot:\n%q\nwant:\n%q", string(content), expected)
	}
}

// TestRemoveEntries_CleanRemoval は、マーカーブロックの完全削除をテストする
func TestRemoveEntries_CleanRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	// 既存内容 + マーカーブロックを書き込み
	initialContent := "127.0.0.1 localhost\n" +
		"::1 localhost\n" +
		"\n" +
		markerStart + "\n" +
		"127.0.0.1 test.localhost\n" +
		markerEnd + "\n"

	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := RemoveEntries(); err != nil {
		t.Fatalf("RemoveEntries failed: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	// 期待される内容: 元の内容のみ（マーカーブロックと前の空行が削除される）
	expected := "127.0.0.1 localhost\n::1 localhost\n"

	if string(content) != expected {
		t.Errorf("unexpected content:\ngot:\n%q\nwant:\n%q", string(content), expected)
	}
}

// TestAddRemoveMultipleTimes は、複数回の追加・削除で空行が累積しないことを確認する
func TestAddRemoveMultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hosts")
	setTestHostsFile(t, testFile)

	// 初期内容を書き込み
	initialContent := "127.0.0.1 localhost\n::1 localhost\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hostnames := []string{"test.localhost", "api.localhost"}

	// 3回繰り返し
	for i := 0; i < 3; i++ {
		// 追加
		if err := AddEntries(hostnames); err != nil {
			t.Fatalf("iteration %d: AddEntries failed: %v", i, err)
		}

		// 削除
		if err := RemoveEntries(); err != nil {
			t.Fatalf("iteration %d: RemoveEntries failed: %v", i, err)
		}

		// 内容を確認
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("iteration %d: failed to read test file: %v", i, err)
		}

		// 期待される内容: 初期内容に戻る（空行が累積しない）
		if string(content) != initialContent {
			t.Errorf("iteration %d: content changed:\ngot:\n%q\nwant:\n%q", i, string(content), initialContent)
		}

		// 空行の数をカウント（デバッグ用）
		lines := strings.Split(string(content), "\n")
		emptyCount := 0
		for _, line := range lines {
			if line == "" {
				emptyCount++
			}
		}

		// 末尾の1つの改行のみが許容される
		if emptyCount > 1 {
			t.Errorf("iteration %d: too many empty lines: %d", i, emptyCount)
		}
	}
}

// TestNormalizeFileEnding は、normalizeFileEnding関数のユニットテスト
func TestNormalizeFileEnding(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "空スライス",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "末尾に空行なし",
			input:    []string{"line1", "line2"},
			expected: []string{"line1", "line2"},
		},
		{
			name:     "末尾に1つの空行",
			input:    []string{"line1", "line2", ""},
			expected: []string{"line1", "line2"},
		},
		{
			name:     "末尾に複数の空行",
			input:    []string{"line1", "line2", "", "", ""},
			expected: []string{"line1", "line2"},
		},
		{
			name:     "すべて空行",
			input:    []string{"", "", ""},
			expected: []string{},
		},
		{
			name:     "途中に空行がある",
			input:    []string{"line1", "", "line2", ""},
			expected: []string{"line1", "", "line2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeFileEnding(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// TestTrimTrailingEmptyLines は、trimTrailingEmptyLines関数のユニットテスト
func TestTrimTrailingEmptyLines(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "空スライス",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "末尾に空行なし",
			input:    []string{"line1", "line2"},
			expected: []string{"line1", "line2"},
		},
		{
			name:     "末尾に1つの空行",
			input:    []string{"line1", "line2", ""},
			expected: []string{"line1", "line2"},
		},
		{
			name:     "末尾に複数の空行",
			input:    []string{"line1", "line2", "", "", ""},
			expected: []string{"line1", "line2"},
		},
		{
			name:     "すべて空行",
			input:    []string{"", "", ""},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimTrailingEmptyLines(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}
