# Frontend Application (Aryan - Member 3)

This directory houses the SvelteKit frontend dashboard for QUANTA.

## Technology Stack
- SvelteKit (Svelte 5 Runes)
- TypeScript
- Modern CSS (Custom Design System, CSS variables, flat design)
- Charting: Chart.js / LayerChart / Apache ECharts

## Interface Contract
The frontend communicates directly with the Go backend on `http://localhost:8080`.
All API schemas adhere to the contracts defined in `shared/schemas/`.

---

## Getting Started

### 1. Prerequisites
- Node.js 20+ (tested with Node 24+)
- npm 10+ (tested with npm 11+)
- Running QUANTA Go backend service on `http://localhost:8080`

### 2. Environment Configuration
Create or inspect `.env` in `apps/frontend/` (sample provided in `.env.example`):
```bash
VITE_API_URL=http://localhost:8080
```

> **Note**: The Go backend CORS middleware allows `http://localhost:3000`. The Vite development server is configured with `server.port = 3000` and `strictPort = true` to match backend CORS policies.

### 3. Installation
Navigate to the frontend directory and install dependencies:
```cmd
cd apps\frontend
npm install
```

### 4. Running the Development Server
```cmd
npm run dev
```
The application will be accessible at:
```
http://localhost:3000
```

### 5. Running with Backend in Docker
To run against the local Docker backend stack:
1. From the repository root, start PostgreSQL, AI decision engine, and the Go backend:
   ```cmd
   docker compose up -d postgres ai-engine backend
   ```
2. Verify backend availability:
   ```cmd
   curl.exe -i http://localhost:8080/api/v1/health
   ```
3. Start the frontend development server:
   ```cmd
   cd apps\frontend
   npm run dev
   ```
4. Open `http://localhost:3000` in your browser.

---

## Architectural Structure & Modules

- `src/lib/api.ts`: Centralized fetch wrapper that auto-attaches `Authorization: Bearer <token>`, unwraps backend error responses (`{"error":"..."}` and `{"error":"validation failed","details":[...]}`), throws structured `ApiError`, and clears storage on `401 Unauthorized` with redirection to `/login`.
- `src/lib/auth.ts`: Reactive Svelte stores (`token`, `user`) synchronized with `localStorage` (guarded by browser execution checks), alongside `login()`, `register()`, `logout()`, and `me()` methods.
- `src/lib/roles.ts`: Role-based authorization helpers (`canWrite(role)`, `isAdmin(role)`) for UI feature gating.
- `src/lib/types.ts`: Exact TypeScript interfaces matching Go backend schemas (`Treasury`, `CreateTreasuryInput`, `TreasuriesListResponse`, `StrategyExecutionLog`, `StrategiesListResponse`, `UserRecord`, `UpdateRoleInput`, `AuditLogRecord`, `AuditLogsListResponse`).
- `src/lib/format.ts`: Currency (en-IN format), percentage, number, and date formatters.
- `src/app.css`: Unified global styling with CSS variables, flat modern palette, accessible buttons, form inputs, badges, data tables, and alerts.
- `src/routes/+layout.ts`: Client-side SPA execution (`ssr = false`, `prerender = false`).
- `src/routes/+layout.svelte`: Persistent top navigation bar displaying branding, navigation links (Dashboard, Treasuries, Users, Audit Logs), user email, role badge, and sign-out control. Enforces client-side route protection against unauthenticated or unauthorized access.
- `src/routes/login/+page.svelte`: Sign-in view with email pattern and password length validation, field-level error messages, loading indicators, and session initialization.
- `src/routes/register/+page.svelte`: Registration view with input validation, password confirmation, duplicate email detection, and redirection to sign-in.
- `src/routes/+page.svelte`: Protected dashboard displaying authenticated user identity, role badge, active capability permissions, live corporate treasuries count, and subsystem overview cards.
- `src/routes/treasuries/+page.svelte`: Paginated corporate treasuries table with Indian number formatting, role-gated onboarding form with field-level validation, and detail page links.
- `src/routes/treasuries/[id]/+page.svelte`: Corporate treasury detail page with full balance sheet metrics, 404 detection, strategy synthesis panel, algorithmic comparison benchmarks, and paginated historical strategy execution logs.
- `src/routes/admin/users/+page.svelte`: Admin user management console displaying registered operator accounts and enabling role reassignments (`admin`, `analyst`, `viewer`) with confirmation dialogs and sole-admin demotion protection.
- `src/routes/admin/audit-logs/+page.svelte`: Admin security audit log viewer displaying paginated, read-only audit records with backend-supported filtering by `action` and `entity_type`.

---

## Diagnostics and Production Build

### SvelteKit Type and Diagnostic Check
```cmd
npm run check
```

### Production Build
```cmd
npm run build
```

### Preview Production Build
```cmd
npm run preview
```
