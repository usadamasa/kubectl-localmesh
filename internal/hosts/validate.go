package hosts

import (
	"fmt"
	"os"
	"strings"
)

// hostsFileState は /etc/hosts ファイルの状態を表す
type hostsFileState struct {
	isValid          bool     // ファイルが有効な状態か（マーカーが存在しない）
	markerBlockCount int      // マーカーブロックの数
	hasUnclosedBlock bool     // 開始マーカーのみで終了マーカーがない
	hasOrphanEnd     bool     // 終了マーカーのみで開始マーカーがない
	hasNestedMarkers bool     // ネストしたマーカー
	problems         []string // 問題の詳細リスト（ユーザー向けメッセージ）
	fileContent      string   // ファイル全体の内容（エラー表示用）
}

// hostsFileCorruptedError は /etc/hosts が無効状態にあることを示すエラー
type hostsFileCorruptedError struct {
	state *hostsFileState
}

// Error implements the error interface
func (e *hostsFileCorruptedError) Error() string {
	var sb strings.Builder
	sb.WriteString("/etc/hosts is in an invalid state and cannot be automatically fixed.\n")
	sb.WriteString("Please manually fix the following problems:\n\n")

	for i, problem := range e.state.problems {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, problem))
	}

	sb.WriteString("\nCurrent /etc/hosts content:\n")
	sb.WriteString("---\n")
	sb.WriteString(e.state.fileContent)
	sb.WriteString("---\n")
	sb.WriteString("\nTo fix:\n")
	sb.WriteString("1. Manually edit /etc/hosts with sudo, like `sudo vim -u NONE /etc/hosts`\n")
	sb.WriteString("2. Remove all lines between and including:\n")
	sb.WriteString(fmt.Sprintf("     %s\n", markerStart))
	sb.WriteString(fmt.Sprintf("     %s\n", markerEnd))
	sb.WriteString("3. Run kubectl-localmesh again\n")

	return sb.String()
}

// newHostsFileCorruptedError creates a new hostsFileCorruptedError
func newHostsFileCorruptedError(state *hostsFileState) error {
	return &hostsFileCorruptedError{state: state}
}

// validateHostsFile は /etc/hosts の状態を検証する
func validateHostsFile() (*hostsFileState, error) {
	state := &hostsFileState{
		isValid:  true,
		problems: []string{},
	}

	// ファイル読み込み
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// ファイルが存在しない = 有効
			return state, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", hostsFile, err)
	}

	state.fileContent = string(content)
	lines := strings.Split(state.fileContent, "\n")

	// マーカーの状態を追跡
	inBlock := false
	blockCount := 0
	startLineNumber := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1 // 1-indexed for user display

		switch trimmed {
		case markerStart:
			if inBlock {
				// ネストした開始マーカー
				state.hasNestedMarkers = true
				state.problems = append(state.problems,
					fmt.Sprintf("Nested start marker found at line %d (block started at line %d)", lineNum, startLineNumber))
			}
			inBlock = true
			startLineNumber = lineNum
			blockCount++
		case markerEnd:
			if !inBlock {
				// 孤立した終了マーカー
				state.hasOrphanEnd = true
				state.problems = append(state.problems,
					fmt.Sprintf("End marker without start marker found at line %d", lineNum))
			}
			inBlock = false
		}
	}

	// ファイル終端で未完結
	if inBlock {
		state.hasUnclosedBlock = true
		state.problems = append(state.problems,
			fmt.Sprintf("Unclosed block: start marker at line %d has no matching end marker", startLineNumber))
	}

	state.markerBlockCount = blockCount

	// マーカーブロックが存在する場合は無効（ユーザー要件）
	if blockCount >= 1 {
		if blockCount == 1 && !state.hasUnclosedBlock && !state.hasOrphanEnd && !state.hasNestedMarkers {
			// 正常なブロック1つ = 前回のクリーンアップ失敗
			state.problems = append([]string{
				fmt.Sprintf("Existing kubectl-localmesh entries found (%d block). Clean shutdown may have failed.", blockCount),
			}, state.problems...)
		} else if blockCount > 1 {
			// 複数ブロック
			state.problems = append([]string{
				fmt.Sprintf("Multiple marker blocks found (%d blocks). Only one is expected.", blockCount),
			}, state.problems...)
		}
	}

	// 有効性判定
	if blockCount > 0 || state.hasUnclosedBlock || state.hasOrphanEnd || state.hasNestedMarkers {
		state.isValid = false
	}

	return state, nil
}
