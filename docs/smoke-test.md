# Manual smoke test

End-to-end verification of the auth + deploy path after a fresh stack bring-up. Run this
after completing the bootstrap steps in [cloud-providers.md](cloud-providers.md)
(Operations → First-run bootstrap sequence), or against any running stack.

## Steps

1. **Bootstrap** — while zero users exist, create the admin user:

   ```bash
   curl -X POST https://<api-host>/auth/bootstrap \
     -H 'Content-Type: application/json' \
     -d '{"email":"you@example.com"}'
   ```

   Save the returned raw token (`yok_...`) immediately — it is shown once.

2. **Login** — authenticate the CLI with that token:

   ```bash
   yok login
   # Paste your Yok token (yok_...):
   ```

   Expect `[OK] Logged in.`

3. **Create project**:

   ```bash
   yok create
   ```

   Expect `[OK] Project ID saved for future deployments`. Re-run `yok login` afterwards if
   you want to double-check that re-login preserves the stored project ID.

4. **Deploy** — from a small Git repository (e.g. any framework starter):

   ```bash
   yok ship
   # or: yok deploy
   ```

5. **Follow logs** — stream build output until completion:

   ```bash
   yok logs --follow
   ```

6. **Verify status transitions** — during the run and at the end:

   ```bash
   yok status
   ```

   Status should progress through the active states (PENDING/QUEUED/IN_PROGRESS) and end
   at **COMPLETED** (or **FAILED** if the build breaks), with a public/deployment URL shown
   on success.

7. **Cancel on terminal state returns an error** — attempt to cancel the finished
   deployment by its ID:

   ```bash
   yok cancel <deployment-id>
   ```

   Because the deployment is no longer active, the API rejects it. Expect
   `Failed to cancel deployment: ... Cannot cancel deployment (...)` rather than a success
   message.

## Known caveat: log/status ordering

The build server publishes log lines and status updates as separate Kafka messages, and
Kafka gives no ordering guarantees across partitions. As a result, the **COMPLETED** status
can be recorded before the last few log lines are stored — so `yok logs` may briefly miss
the tail of the output even though the deployment already reports COMPLETED.
