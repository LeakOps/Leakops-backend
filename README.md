# LeakOps Backend

LeakOps monitors failed payments, dunning, proration errors, coupon abuse and more — so you can recover lost revenue automatically.

This repository is the Go (Fiber) backend for LeakOps.

## Tech Stack

- **Language:** Go
- **Web Framework:** [Fiber](https://gofiber.io/)
- **ORM:** GORM
- **Database:** PostgreSQL
- **Payment Gateways Monitored:** Stripe, Dodo Payments
- **Billing (LeakOps's own subscriptions):** Dodo Payments
- **Email:** Resend
- **File Storage:** Supabase Storage (S3-compatible)
- **Auth:** JWT, Google OAuth, GitHub OAuth
- **Background Jobs:** Custom retry engine (Day 1/3/7 schedule)
- **Email** Resend (for dunning and notifications)

## Project Structure

```
Leakops-backend/
├── internal/
│   ├── config/         # Environment variable loading
│   ├── db/              # Database connection + auto-migration
│   ├── email/            # Dunning + notification emails (Resend)
│   ├── gateway/          # Stripe/Dodo webhook verification, retry logic
│   ├── handlers/         # HTTP request handlers
│   ├── middlewares/      # Auth (JWT) and CORS middleware
│   ├── models/           # Database models (GORM)
│   ├── retry/            # Background retry engine (Day 1/3/7 schedule)
│   ├── routes/           # Route registration
│   ├── services/         # LeakOps's own billing + storage services
│   └── utils/            # Shared helpers (encryption, webhook signature verification)
├── server/
│   └── main.go           # Entry point
├── docker-compose.yml    # Local PostgreSQL for development
├── go.mod
└── README.md
```

## Prerequisites

Before setting up the project locally, make sure you have:

- [Go](https://go.dev/dl/) 1.21 or later
- [Docker](https://www.docker.com/) (for running PostgreSQL locally)
- A code editor (VS Code recommended)
- Accounts on the following services (all have free tiers):
  - [Stripe](https://stripe.com) (Sandbox/Test mode)
  - [Dodo Payments](https://dodopayments.com) (Test mode)
  - [Resend](https://resend.com) (for sending emails)
  - [Supabase](https://supabase.com) (for file storage)
  - [Google Cloud Console](https://console.cloud.google.com) (for Google OAuth)
  - [GitHub Developer Settings](https://github.com/settings/developers) (for GitHub OAuth)

## Getting Started (Fork & Local Setup)


### 1. Fork and Clone the Repository

```bash
git clone https://github.com/<your-username>/Leakops-backend.git
cd Leakops-backend
```

### 2. Start the Local Database

This project uses Docker Compose to run a local PostgreSQL instance for development.

```bash
docker compose up -d
```

This starts a PostgreSQL container based on the credentials defined in your `.env` file (see below).

### 3. Set Up Environment Variables

Copy the example file and fill in your own values:

```bash
cp .env.example .env
```

Fill in the following variables in `.env`:

```dotenv
# Server
PORT=8080
FRONTEND_URL=http://localhost:3000
BASE_URL=http://127.0.0.1:8080

# Auth
JWT_SECRET=your-random-secret-key
ENCRYPTION_KEY=your-32-character-encryption-key

# PostgreSQL (must match docker-compose.yml)
POSTGRES_USER=XXXXXXX
POSTGRES_PASSWORD=XXXXXXX
POSTGRES_DB=XXXXXX
DATABASE_URL=postgres://XXXXXXXXXX

# Google OAuth
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/google/callback

# GitHub OAuth
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
GITHUB_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/github/callback

# Resend (email)
RESEND_API_KEY=
RESEND_FROM_EMAIL=

# Supabase Storage (S3-compatible, for profile pictures)
SUPABASE_S3_ENDPOINT=
SUPABASE_S3_REGION=
SUPABASE_S3_ACCESS_KEY_ID=
SUPABASE_S3_SECRET_ACCESS_KEY=
SUPABASE_BUCKET_NAME=
SUPABASE_PUBLIC_URL=

# LeakOps's own Dodo account (for billing/subscriptions)
LEAKOPS_DODO_API_KEY=
LEAKOPS_DODO_ENVIRONMENT=test
LEAKOPS_DODO_WEBHOOK_SECRET=
DODO_PRODUCT_STARTER=
DODO_PRODUCT_GROWTH=
DODO_PRODUCT_SCALE=
```

**Note:** Founders connect their own Stripe/Dodo API keys through the app itself (via `/gateway/connect`) — these are never stored in `.env`. Only LeakOps's own billing account needs a key in `.env`.

**Important:** `ENCRYPTION_KEY` must be exactly 32 characters (used for AES-256 encryption of stored gateway credentials). Generate one with:

```bash
python3 -c "import secrets, string; print(''.join(secrets.choice(string.ascii_letters + string.digits) for _ in range(32)))"
```

### 4. Install Dependencies

```bash
go mod tidy
```

### 5. Run the Server

```bash
go run server/main.go
```

On success, you should see:

```
Database connected successfully
Migration completed successfully!!
Retry engine started
Leakops backend listening on port :8080
```

The API is now available at `http://127.0.0.1:8080/api/v1`.

### 6. Testing Webhooks Locally

Since Stripe/Dodo need a public URL to send webhooks to, use [ngrok](https://ngrok.com/) during local development:

```bash
ngrok http 8080
```

Update `BASE_URL` in your `.env` with the ngrok URL, then restart the server. Founder-connected gateway webhooks are auto-registered when a gateway is connected via `/gateway/connect` — no manual webhook setup is needed for that flow.

## Key API Endpoints

All endpoints are prefixed with `/api/v1`.

| Endpoint | Description |
|---|---|
| `POST /auth/signup`, `POST /auth/login` | Email/password auth |
| `GET /auth/google`, `GET /auth/github` | OAuth login |
| `POST /gateway/connect` | Connect a Stripe/Dodo account |
| `GET /dashboard/summary` | Revenue-at-risk and recovery stats |
| `GET /dashboard/payments` | List of failed payments |
| `GET /dashboard/export` | CSV export of failed payments |
| `POST /billing/checkout` | Start a LeakOps subscription checkout |
| `POST /profile/picture` | Upload a profile picture |
| `POST /feedback` | Submit product feedback |

## How to Contribute

1. **Fork** this repository and clone your fork locally.
2. **Create a new branch** for your change:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. **Follow the existing code structure and conventions** — handlers go in `internal/handlers/`, routes in `internal/routes/`, models in `internal/models/`, etc. Keep new gateway integrations behind the `PaymentGateway` interface in `internal/gateway/interface.go`.
4. **Test your changes locally** before submitting — make sure `go build ./...` passes and manually verify the affected endpoints with a tool like Postman.
5. **Do not commit your `.env` file** or any real API keys/secrets. Only commit changes to `.env.example` if you add new required variables.
6. **Open a Pull Request** against the `main` branch with a clear description of what you changed and why.
7. For significant changes (new features, architecture changes), please open an issue first to discuss before submitting a PR.

## License

Specify your license here (e.g. MIT, Apache 2.0, or "All rights reserved" if proprietary).
