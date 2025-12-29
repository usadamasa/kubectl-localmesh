package hosts

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var hostsFile = "/etc/hosts"

const markerStart = "# kubectl-local-mesh: managed by kubectl-local-mesh"
const markerEnd = "# kubectl-local-mesh: end"

// HasPermission checks if we can write to /etc/hosts
func HasPermission() bool {
	// /etc/hostsへの書き込み権限チェック
	f, err := os.OpenFile(hostsFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// AddEntries adds hostname entries to /etc/hosts
func AddEntries(hostnames []string) error {
	// 既存のエントリを削除（古いエントリがあれば）
	if err := RemoveEntries(); err != nil {
		return fmt.Errorf("failed to clean up old entries: %w", err)
	}

	// 現在のファイル内容を読み込み、正規化
	lines, err := readAndNormalizeFile()
	if err != nil {
		return err
	}

	// ファイルが空でない場合、1行の空行で区切る
	if len(lines) > 0 {
		lines = append(lines, "")
	}

	// マーカー開始（直接追加、先頭に改行を入れない）
	lines = append(lines, markerStart)

	// 各ホスト名のエントリを追加
	for _, hostname := range hostnames {
		entry := fmt.Sprintf("127.0.0.1 %s", hostname)
		lines = append(lines, entry)
	}

	// マーカー終了
	lines = append(lines, markerEnd)

	// ファイルに書き込み
	return writeLinesToFile(lines)
}

// RemoveEntries removes kubectl-local-mesh entries from /etc/hosts
func RemoveEntries() error {
	// /etc/hostsを読み込む
	f, err := os.Open(hostsFile)
	if err != nil {
		// ファイルが存在しない場合は何もしない（エラーではない）
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open %s: %w", hostsFile, err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	inManagedBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == markerStart {
			inManagedBlock = true
			// マーカー開始の直前の空行を削除
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			continue
		}

		if strings.TrimSpace(line) == markerEnd {
			inManagedBlock = false
			continue
		}

		// 管理対象ブロック外の行のみ保持
		if !inManagedBlock {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// 末尾の空行を正規化
	lines = normalizeFileEnding(lines)

	// ファイルに書き戻す
	return writeLinesToFile(lines)
}

// trimTrailingEmptyLines は、スライスの末尾にある全ての空行を削除する
func trimTrailingEmptyLines(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// normalizeFileEnding は、末尾の空行を削除する
// ファイルに書き込む際に各行に"\n"が追加されるため、ここでは末尾の空行を削除するのみ
func normalizeFileEnding(lines []string) []string {
	return trimTrailingEmptyLines(lines)
}

// readAndNormalizeFile は、hostsファイルを読み込み、末尾を正規化する
func readAndNormalizeFile() ([]string, error) {
	f, err := os.Open(hostsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to open %s: %w", hostsFile, err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return normalizeFileEnding(lines), nil
}

// writeLinesToFile は、行のスライスをhostsファイルにアトミックに書き込む
func writeLinesToFile(lines []string) error {
	tmpFile := hostsFile + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile) }()

	for _, line := range lines {
		if _, err := out.WriteString(line + "\n"); err != nil {
			_ = out.Close()
			return err
		}
	}

	if err := out.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpFile, hostsFile); err != nil {
		return fmt.Errorf("failed to replace %s: %w", hostsFile, err)
	}

	return nil
}
