# Source from the repository root before running Go commands.
export GONOPROXY=github.com/sihuangtech/probelune-contracts
export GONOSUMDB=github.com/sihuangtech/probelune-contracts
# Re-scan source files when a maintained tag keeps the same module path.
export GODEBUG="${GODEBUG:+$GODEBUG,}goindex=0"
export GOMODCACHE="$PWD/.cache/go-mod-v0.1.0"

# The committed checksum owns this project's contracts cache.
probelune_sumfile="$PWD/go.sum"
if [ ! -f "$probelune_sumfile" ]; then probelune_sumfile="$PWD/server/go.sum"; fi
probelune_expected=$(awk '$1 == "github.com/sihuangtech/probelune-contracts" && $2 == "v0.1.0" {print $3}' "$probelune_sumfile")
probelune_download="$GOMODCACHE/cache/download/github.com/sihuangtech/probelune-contracts"
probelune_source="$GOMODCACHE/github.com/sihuangtech/probelune-contracts@v0.1.0"
if [ -n "$probelune_expected" ] && [ -f "$probelune_download/@v/v0.1.0.ziphash" ] && [ "$(cat "$probelune_download/@v/v0.1.0.ziphash")" != "$probelune_expected" ]; then
  if [ -d "$probelune_source" ]; then chmod -R u+w "$probelune_source"; fi
  rm -rf "$probelune_source" "$probelune_download" "$GOMODCACHE/cache/vcs"
fi
unset probelune_sumfile probelune_expected probelune_download probelune_source
