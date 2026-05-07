#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OPENAPI="$ROOT_DIR/docs/openapi.yaml"
APIFOX="$ROOT_DIR/docs/apifox-client-openapi.yaml"
python3 - <<'PY' "$OPENAPI" "$APIFOX"
import sys
from pathlib import Path
import yaml
openapi=yaml.safe_load(Path(sys.argv[1]).read_text(encoding='utf-8'))
apifox=yaml.safe_load(Path(sys.argv[2]).read_text(encoding='utf-8'))
required={'/health','/client/ping','/client/bootstrap','/users/register','/users/login','/users/profile','/users/profile/avatar','/users/online/session/start','/users/online/heartbeat','/users/online/status','/users/online/logout','/files','/file','/stream','/get_music','/music/artist','/music/search','/music/health-test','/external/music/jamendo/search','/external/music/jamendo/track','/artist/search','/user/favorites/add','/user/favorites','/user/favorites/remove','/user/history/add','/user/history','/user/history/delete','/user/history/clear','/user/playlists','/user/playlists/{playlist_id}','/user/playlists/{playlist_id}/update','/user/playlists/{playlist_id}/delete','/user/playlists/{playlist_id}/items/add','/user/playlists/{playlist_id}/items/remove','/user/playlists/{playlist_id}/items/reorder','/music/comments','/music/comments/{comment_id}/replies','/music/comments/{comment_id}/delete','/music/charts/hot','/recommendations/audio','/recommendations/similar/{song_id}','/recommendations/feedback','/upload','/files/{path}','/download','/lrc','/uploads/{folder}/lrc','/uploads/{path}','/music/local/seek-index','/music/local/playback-info','/videos','/video/stream','/video/{path}','/users/add_music'}
forbidden={'/','/records','/add','/stats','/ack'}
openapi_paths=set(openapi.get('paths',{})); apifox_paths=set(apifox.get('paths',{}))
missing_openapi=sorted(required-openapi_paths); missing_apifox=sorted(required-apifox_paths); forbidden_apifox=sorted(p for p in apifox_paths if p in forbidden or p.startswith('/admin'))
if missing_openapi or missing_apifox or forbidden_apifox:
    print('missing_openapi=', missing_openapi); print('missing_apifox=', missing_apifox); print('forbidden_apifox=', forbidden_apifox); raise SystemExit(1)
if not any(isinstance(s,dict) and s.get('url')=='http://192.168.1.208:8080' for s in apifox.get('servers',[])):
    print('missing VM server'); raise SystemExit(1)
print(f'openapi client scope ok: {len(apifox_paths)} Apifox paths')
PY
