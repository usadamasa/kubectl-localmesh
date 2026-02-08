#!/bin/bash
# GCP SSH Tunnel 診断スクリプト
# SSH Bastion経由のDB接続に必要なコンポーネントを診断します

set -e

echo "========================================"
echo "GCP SSH Tunnel 診断"
echo "========================================"
echo ""

warnings=0
errors=0

# ─── gcloud CLI ─────────────────────────

echo "【gcloud CLI】"
if ! command -v gcloud &> /dev/null; then
    echo "  ❌ gcloud コマンドが見つかりません"
    echo "     インストール方法: https://cloud.google.com/sdk/docs/install"
    errors=$((errors + 1))
else
    version=$(gcloud version 2>/dev/null | head -n 1)
    echo "  ✅ $version"

    # アクティブアカウント確認
    account=$(gcloud config get-value account 2>/dev/null || true)
    if [ -n "$account" ] && [ "$account" != "(unset)" ]; then
        echo "     アクティブアカウント: $account"
    else
        echo "  ⚠️  アクティブアカウントが設定されていません"
        echo "     設定方法: gcloud auth login"
        warnings=$((warnings + 1))
    fi
fi
echo ""

# ─── ADC認証トークン ────────────────────

echo "【Application Default Credentials (ADC)】"
if [ -n "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
    if [ -f "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
        echo "  ✅ 環境変数GOOGLE_APPLICATION_CREDENTIALSが設定されています"
        echo "     ファイル: $GOOGLE_APPLICATION_CREDENTIALS"
    else
        echo "  ❌ GOOGLE_APPLICATION_CREDENTIALSが設定されていますが、ファイルが存在しません"
        echo "     ファイル: $GOOGLE_APPLICATION_CREDENTIALS"
        errors=$((errors + 1))
    fi
elif [ -f "$HOME/.config/gcloud/application_default_credentials.json" ]; then
    echo "  ✅ Application Default Credentialsが設定されています"
    echo "     ファイル: $HOME/.config/gcloud/application_default_credentials.json"
else
    echo "  ❌ Application Default Credentialsが見つかりません"
    echo "     設定方法: gcloud auth application-default login"
    errors=$((errors + 1))
fi

# ADCトークン取得テスト
if command -v gcloud &> /dev/null; then
    if token=$(gcloud auth application-default print-access-token 2>/dev/null); then
        echo "  ✅ ADCトークン取得成功"
    else
        echo "  ⚠️  ADCトークン取得失敗"
        echo "     再認証: gcloud auth application-default login"
        warnings=$((warnings + 1))
    fi
fi
echo ""

# ─── SSH鍵ファイル ──────────────────────

echo "【SSH鍵ファイル】"
found_key=false

# google_compute_engine (gcloudデフォルト)
if [ -f "$HOME/.ssh/google_compute_engine" ]; then
    echo "  ✅ ~/.ssh/google_compute_engine"
    found_key=true

    # 証明書ファイル確認
    if [ -f "$HOME/.ssh/google_compute_engine-cert.pub" ]; then
        echo "     📜 証明書: ~/.ssh/google_compute_engine-cert.pub"
        # 有効期限確認(ssh-keygenがあれば)
        if command -v ssh-keygen &> /dev/null; then
            validity=$(ssh-keygen -L -f "$HOME/.ssh/google_compute_engine-cert.pub" 2>/dev/null | grep "Valid:" || true)
            if [ -n "$validity" ]; then
                echo "     $validity"
            fi
        fi
    fi
else
    echo "  ℹ️  ~/.ssh/google_compute_engine (なし - gcloudデフォルト)"
fi

# id_ed25519
if [ -f "$HOME/.ssh/id_ed25519" ]; then
    echo "  ✅ ~/.ssh/id_ed25519"
    found_key=true
else
    echo "  ℹ️  ~/.ssh/id_ed25519 (なし)"
fi

# id_rsa
if [ -f "$HOME/.ssh/id_rsa" ]; then
    echo "  ✅ ~/.ssh/id_rsa"
    found_key=true
else
    echo "  ℹ️  ~/.ssh/id_rsa (なし)"
fi

if [ "$found_key" = false ]; then
    echo ""
    echo "  ⚠️  SSH秘密鍵が見つかりません(--experimental-ssh使用時に必要)"
    echo "     生成方法: gcloud compute ssh INSTANCE --zone=ZONE --project=PROJECT --tunnel-through-iap"
    warnings=$((warnings + 1))
fi
echo ""

# ─── ssh-agent ──────────────────────────

echo "【ssh-agent】"
if [ -n "$SSH_AUTH_SOCK" ]; then
    echo "  ✅ SSH_AUTH_SOCK=$SSH_AUTH_SOCK"
    if ssh-add -l &> /dev/null; then
        key_count=$(ssh-add -l 2>/dev/null | wc -l | tr -d ' ')
        echo "     登録鍵数: $key_count"
    else
        echo "  ⚠️  ssh-agentに鍵が登録されていません"
        echo "     登録方法: ssh-add ~/.ssh/google_compute_engine"
        warnings=$((warnings + 1))
    fi
else
    echo "  ℹ️  SSH_AUTH_SOCK が設定されていません(ssh-agent未使用)"
fi

# sudo環境でのSSH_AUTH_SOCK問題チェック
if [ "$(id -u)" -eq 0 ] && [ -z "$SSH_AUTH_SOCK" ]; then
    echo "  ⚠️  root実行中にSSH_AUTH_SOCKが未設定"
    echo "     sudo実行時は SSH_AUTH_SOCK がリセットされます"
    echo "     対策: sudo SSH_AUTH_SOCK=\$SSH_AUTH_SOCK kubectl-localmesh ..."
    warnings=$((warnings + 1))
fi
echo ""

# ─── OS Login ───────────────────────────

echo "【OS Login (POSIXユーザー名)】"
if command -v gcloud &> /dev/null; then
    profile=$(gcloud compute os-login describe-profile 2>/dev/null || true)
    if [ -n "$profile" ]; then
        username=$(echo "$profile" | grep "username:" | head -n 1 | awk '{print $2}')
        if [ -n "$username" ]; then
            echo "  ✅ POSIXユーザー名: $username"
        else
            echo "  ⚠️  POSIXアカウントが見つかりません"
            echo "     gcloud compute ssh で初回接続すると自動作成されます"
            warnings=$((warnings + 1))
        fi
    else
        echo "  ℹ️  OS Login プロファイル取得不可(OS Login無効の可能性)"
    fi
else
    echo "  ℹ️  gcloud未インストールのためスキップ"
fi
echo ""

# ─── 結果サマリー ───────────────────────

echo "========================================"
echo "診断結果サマリー"
echo "========================================"
echo ""

if [ $errors -eq 0 ] && [ $warnings -eq 0 ]; then
    echo "✅ すべてのチェックが通過しました"
    echo ""
    echo "次のステップ:"
    echo "  # gcloud CLI版(デフォルト)"
    echo "  sudo kubectl-localmesh up -f services.yaml"
    echo ""
    echo "  # Go SDK版(experimental)"
    echo "  sudo kubectl-localmesh up -f services.yaml --experimental-ssh"
elif [ $errors -eq 0 ]; then
    echo "⚠️  警告: ${warnings}件"
    echo "   上記の警告を確認してください(動作に影響する可能性があります)"
else
    echo "❌ エラー: ${errors}件 / 警告: ${warnings}件"
    echo "   エラーを解決してから再実行してください"
fi
echo "========================================"

exit $errors
