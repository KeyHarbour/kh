# KeyHarbour Backend API Implementation Prompt

Use this prompt on your Rails backend project to implement the API endpoints required for the `kh migrate` CLI command.

---

## Prompt for Backend Implementation

I need to implement a REST API for my KeyHarbour Terraform state management backend (Rails). The CLI tool `kh` requires these endpoints to support the `kh migrate auto` command which migrates Terraform state from Terraform Cloud to KeyHarbour.

### Required API Endpoints

Implement the following JSON API endpoints under the `/v1/` namespace:

#### 1. Project Endpoints

**GET /v1/projects/:uuid**
- Returns project details by UUID
- Used to validate project exists before migration
- Response (200 OK):
```json
{
  "uuid": "project-uuid-here",
  "name": "My Project",
  "description": "Optional description",
  "environment_names": ["dev", "staging", "prod"]
}
```
- Return 404 if project not found

#### 2. Workspace Endpoints

**GET /v1/projects/:project_uuid/workspaces**
- Lists all workspaces within a project
- Response (200 OK):
```json
[
  {"uuid": "workspace-uuid-1", "name": "default", "description": ""},
  {"uuid": "workspace-uuid-2", "name": "prod", "description": "Production"}
]
```

**POST /v1/projects/:project_uuid/workspaces** ⚠️ NEW - REQUIRED FOR BULK MIGRATION
- Create a new workspace in a project
- Request body:
```json
{
  "workspace": {
    "name": "cli-migration-test"
  }
}
```
- Response (201 Created):
```json
{
  "uuid": "new-workspace-uuid",
  "name": "cli-migration-test"
}
```
- Returns 422 if workspace name already exists in the project

**GET /v1/projects/:project_uuid/workspaces/:workspace_uuid**
- Get workspace details by UUID
- Response (200 OK):
```json
{
  "uuid": "workspace-uuid",
  "name": "prod",
  "description": "Production workspace"
}
```
- Return 404 if workspace not found

#### 3. Statefile Endpoints (Critical for Migration)

**GET /v1/projects/:project_uuid/workspaces/:workspace_uuid/statefiles**
- List statefiles for a workspace
- Query params: `environment` (optional filter)
- Response (200 OK):
```json
[
  {
    "uuid": "statefile-uuid",
    "content": "{\"version\":4,...}",
    "published_at": "2025-12-21T12:00:00Z",
    "environment": "prod"
  }
]
```
- Return empty array `[]` if none exist (not 404)

**GET /v1/projects/:project_uuid/workspaces/:workspace_uuid/statefiles/last**
- Get the most recent statefile
- Query params: `environment` (optional filter)
- Response (200 OK): Same as single statefile object
- Return 404 if no statefiles exist

**GET /v1/projects/:project_uuid/workspaces/:workspace_uuid/statefiles/:uuid**
- Get specific statefile by UUID
- Response (200 OK): Single statefile object
- Return 404 if not found

**POST /v1/projects/:project_uuid/workspaces/:workspace_uuid/statefiles**
- Create/upload a new statefile (THIS IS THE MAIN MIGRATION ENDPOINT)
- Query params: `environment` (optional tag)
- Request body:
```json
{
  "content": "{\"version\":4,\"terraform_version\":\"1.13.4\",\"serial\":1,\"lineage\":\"...\",\"outputs\":{},\"resources\":[...]}"
}
```
- Response (201 Created or 200 OK):
```json
{
  "status": "created"
}
```
- The `content` field contains the raw Terraform state JSON as a string

**DELETE /v1/projects/:project_uuid/workspaces/:workspace_uuid/statefiles**
- Delete all statefiles in a workspace
- Response: 204 No Content

**DELETE /v1/projects/:project_uuid/workspaces/:workspace_uuid/statefiles/:uuid**
- Delete specific statefile
- Response: 204 No Content

#### 4. Terraform HTTP Backend Endpoints (For Terraform CLI integration)

These endpoints are used when Terraform itself runs `terraform init/plan/apply` against KeyHarbour:

**GET /v1/projects/:project_uuid/workspaces/:workspace_uuid/state**
- Read current state (raw Terraform state JSON)
- Headers: `Accept: application/vnd.terraform.state+json;version=4`
- Response: Raw JSON state file content (Content-Type: application/json)
- Return 404 or empty state if none exists

**POST /v1/projects/:project_uuid/workspaces/:workspace_uuid/state**
- Write/update state
- Headers: `Content-Type: application/json`
- Body: Raw Terraform state JSON
- Response: 200 OK or 201 Created

**POST /v1/projects/:project_uuid/workspaces/:workspace_uuid/state/lock**
- Acquire state lock
- Body (Terraform lock info):
```json
{
  "ID": "lock-id",
  "Operation": "OperationTypePlan",
  "Who": "user@host",
  "Version": "1.5.0",
  "Created": "2025-12-21T12:00:00Z"
}
```
- Response: 200 OK if locked, 409 Conflict if already locked

**DELETE /v1/projects/:project_uuid/workspaces/:workspace_uuid/state/lock**
- Release state lock
- Query params: `force=true` (optional, force unlock)
- Response: 200 OK or 204 No Content

#### 5. Legacy State Endpoints (Optional, for backward compatibility)

**GET /v1/states**
- List all states (for `kh state ls`)
- Query params: `project`, `module`, `workspace` (optional filters)
- Response (200 OK):
```json
[
  {
    "id": "myapp-infra-prod",
    "project": "myapp",
    "module": "infra",
    "workspace": "prod",
    "lineage": "...",
    "serial": 5,
    "size": 2160,
    "checksum": "sha256:...",
    "created_at": "2025-12-21T12:00:00Z"
  }
]
```

**GET /v1/states/:id**
- Get raw state by ID
- Headers: `Accept: application/vnd.terraform.state+json;version=4`
- Response header: `X-State-Meta` (JSON with metadata)
- Response body: Raw Terraform state JSON

### Authentication

All endpoints require Bearer token authentication:
```
Authorization: Bearer <token>
```

The token is obtained via `kh login` and stored in config. Your backend should validate this token and scope access to the authenticated user/organization.

### Implementation Priority

For `kh migrate auto` to work, implement in this order:
1. **GET /v1/projects/:uuid** - Project validation
2. **GET /v1/projects/:project_uuid/workspaces** - Workspace listing
3. **GET /v1/projects/:project_uuid/workspaces/:workspace_uuid** - Workspace lookup
4. **POST /v1/projects/:project_uuid/workspaces/:workspace_uuid/statefiles** - State upload (most critical)
5. **GET /v1/projects/:project_uuid/workspaces/:workspace_uuid/statefiles/last** - Post-migration validation

### Rails Route Suggestions

```ruby
namespace :api do
  namespace :v1 do
    resources :projects, only: [:show] do
      resources :workspaces, only: [:index, :show] do
        resources :statefiles, only: [:index, :show, :create, :destroy]
        get 'statefiles/last', to: 'statefiles#last'
        
        # Terraform HTTP backend
        resource :state, only: [:show, :create], controller: 'terraform_state' do
          post 'lock'
          delete 'lock', action: :unlock
        end
      end
    end
    
    # Legacy flat state endpoints
    resources :states, only: [:index, :show]
  end
end
```

### Error Response Format

Return errors as JSON:
```json
{
  "error": "Not found",
  "message": "Project with UUID xyz not found"
}
```

With appropriate HTTP status codes: 400, 401, 403, 404, 409, 500.

---

## Testing the Implementation

After implementing, test with:

```bash
# Set up environment
export KH_ENDPOINT=http://your-server:3000
export KH_TOKEN=your-token
export KH_PROJECT=your-project-uuid

# Test project lookup
curl -H "Authorization: Bearer $KH_TOKEN" "$KH_ENDPOINT/v1/projects/$KH_PROJECT"

# Test migration
kh migrate auto -d /path/to/terraform/project --project=$KH_PROJECT --dry-run
```
