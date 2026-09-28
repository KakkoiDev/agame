# AGame browser build

AGame runs the canonical Go engine as WASM. OPFS is the live store. Each universe is a Git branch. GitHub is optional backup/synchronization.

Build:

```sh
GOOS=js GOARCH=wasm go build -o browser/agame.wasm ./cmd/agame-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" browser/wasm_exec.js
cd browser
npm install
npm run build
```

Serve `browser/` from HTTPS or localhost. The service worker caches the application shell, so an already-loaded installation can start offline.

The current GitHub connection UI accepts a credential for the browser session only; it is never written into OPFS, Git commits, exports, or the remote URL. A provider OAuth/device-flow UX can replace this credential input without changing universe storage semantics.

## Boundary

AGame owns the `universe/*` convention and game-library UI. Generic OPFS/isomorphic-git behavior follows Jikko's browser Git adapter contract. GitHub backup vendors Jikko's `GitHubRemote` transport, which uses GitHub's Git Database REST API so the static app does not require a credential-bearing CORS proxy. Provider-specific authentication belongs at the AGame edge.
