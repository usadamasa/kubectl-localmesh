package hosts

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const hostsFile = "/etc/hosts"
const markerStart = "# kubectl-local-mesh: managed by kubectl-local-mesh"
const markerEnd = "# kubectl-local-mesh: end"

// HasPermission checks if we can write to /etc/hosts
func HasPermission() bool {
	// /etc/hostsへの書き込み権限チェック
	f, err := os.OpenFile(hostsFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// AddEntries adds hostname entries to /etc/hosts
func AddEntries(hostnames []string) error {
	// 既存のエントリを削除（古いエントリがあれば）
	if err := RemoveEntries(); err != nil {
		return fmt.Errorf("failed to clean up old entries: %w", err)
	}

	// 新しいエントリを追加
	f, err := os.OpenFile(hostsFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", hostsFile, err)
	}
	defer f.Close()

	// マーカー開始
	if _, err := f.WriteString("\n" + markerStart + "\n"); err != nil {
		return err
	}

	// 各ホスト名のエントリを追加
	for _, hostname := range hostnames {
		entry := fmt.Sprintf("127.0.0.1 %s\n", hostname)
		if _, err := f.WriteString(entry); err != nil {
			return err
		}
	}

	// マーカー終了
	if _, err := f.WriteString(markerEnd + "\n"); err != nil {
		return err
	}

	return nil
}

// RemoveEntries removes kubectl-local-mesh entries from /etc/hosts
func RemoveEntries() error {
	// /etc/hostsを読み込む
	f, err := os.Open(hostsFile)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", hostsFile, err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	inManagedBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == markerStart {
			inManagedBlock = true
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

	// /etc/hostsに書き戻す
	tmpFile := hostsFile + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	for _, line := range lines {
		if _, err := out.WriteString(line + "\n"); err != nil {
			out.Close()
			os.Remove(tmpFile)
			return err
		}
	}

	if err := out.Close(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	// 一時ファイルを/etc/hostsに上書き
	if err := os.Rename(tmpFile, hostsFile); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to replace %s: %w", hostsFile, err)
	}

	return nil
}
