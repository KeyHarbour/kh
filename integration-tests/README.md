# KeyHarbour CLI – Integration Test Scenarios

This folder documents how to exercise the CLI end-to-end against a live KeyHarbour backend (eg. the Rails app in `../app`). The idea is to run the real API (locally or in a dev environment), configure the CLI to point at it, and execute representative flows.

---

## 1. Prerequisites

1. **Backend running locally**
   ```bash
   cd ../app
   bundle install
   bin/rails db:setup          # once
   bin/dev                     # or `bundle exec puma -C config/puma.rb`
   ```
   By default the API listens on `https://api.keyharbour.test`. Ensure your `puma-dev` or TLS proxy is configured (see `app/README.md`). Typical setup:
   
   ```bash
   brew install puma/puma/puma-dev
   sudo puma-dev -setup
   ln -s "$PWD" ~/Library/Application\ Support/io.puma.dev/api.keyharbour
   puma-dev -restart
   ```
   
   Then start the backend with `puma-dev -start` (or `brew services start puma-dev`), so `https://api.keyharbour.test` resolves locally.

2. **CLI compiled**
   ```bash
   make tidy
   make build
   sudo make install   # installs /usr/local/bin/kh
   kh --help
   ```

3. **Environment**
   - Export `KH_ENDPOINT=https://api.keyharbour.test`
   - Export `KH_TOKEN=<valid PAT>`
   - Optionally set `KH_PROJECT`, `KH_ORG`, etc.

---

## 2. Quick Smoke Test

```bash
export KH_ENDPOINT=https://api.keyharbour.test
export KH_TOKEN=<PAT>

./kh --version
./kh whoami -o json
```

Expected: version prints and `/v1/auth` is reachable.

---

## 3. State Lifecycle Scenario

0. **Scaffold a demo project/workspace (once)**
   ```bash
   ./kh init project \
     -n demo-app \
     -e dev \
     -m infra \
     --dir ../sandbox/demo-app \
     --backend http
   ```
   This bootstraps a Terraform project locally. Deploy or register the project/workspace in KeyHarbour (as needed) so subsequent commands have valid IDs.

1. **List states**
   ```bash
   ./kh state ls -o json
   ```
2. **Create/push a statefile**
   ```bash
   ./kh statefiles push \
     --project <project-name-or-uuid> \
     --workspace <workspace-name-or-uuid> \
     --file ./testdata/example.tfstate \
     --environment dev
   ```
3. **Verify last statefile**
   ```bash
   ./kh statefiles last \
     --project <project> \
     --workspace <workspace> \
     --raw | jq .
   ```
4. **Fetch via `state show`**
   ```bash
   STATE_ID=<id from kh state ls>
   ./kh state show "$STATE_ID" --raw > /tmp/state.json
   jq '.resources | length' /tmp/state.json
   ```
5. **Lock/unlock**
   ```bash
   ./kh lock "$STATE_ID"
   ./kh unlock "$STATE_ID"
   ```

---

## 4. Import/Export Scenario

1. Export an existing state to disk:
   ```bash
   ./kh export tfstate \
     --project <project> \
     --workspace <workspace> \
     --out "out/{workspace}.tfstate"
   ```
2. Import it back via HTTP or local mode:
   ```bash
   ./kh import tfstate \
     --from=local \
     --path out \
     --project <project> \
     --module infra \
     --dry-run
   ```

---

## 5. Failure Handling

- Stop the backend and rerun `kh state ls` to confirm retry + error messaging.
- Use an expired token to ensure 401s are surfaced.

---

## 6. Notes

- Run `KH_DEBUG=1 ./kh …` to capture request/response logs.
- Keep an eye on `log/development.log` in the Rails app to verify the API endpoints being hit.
- When testing destructive actions (statefiles `rm-all`, workspace changes), use a disposable project/workspace.
