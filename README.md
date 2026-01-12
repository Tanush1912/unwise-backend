# Unwise Backend

A production-ready Splitwise alternative backend API built with Go, featuring AI-powered receipt parsing, comprehensive expense management, and advanced settlement calculations.

## Features

### Core Functionality
-  **JWT-based Authentication** - Supabase JWT token validation with ES256/HS256 support
-  **Group Management** - Create, update, and manage expense groups with multiple members
-  **Expense Tracking** - Full CRUD operations for expenses with multiple split methods
-  **Settlement Calculations** - Automatic calculation of who owes whom with edge list representation
-  **Receipt Scanning** - AI-powered receipt parsing using Google Gemini Vision API
-  **Transaction Explanations** - AI-generated explanations for expenses using Gemini
-  **Comments & Reactions** - Social features for expenses with emoji reactions
-  **Friends System** - Manage friend relationships and view cross-group balances
-  **Dashboard** - Comprehensive user dashboard with metrics and recent activity
-  **CSV Import** - Import expenses from Splitwise CSV exports
-  **CSV Export** - Export group transactions to CSV format
-  **Placeholder Users** - Support for non-registered users with claiming functionality
-  **Avatar Management** - Upload and manage user and group avatars
-  **Balance Tracking** - Real-time balance calculations across all groups

### Expense Split Methods
- **EQUAL** - Split expense equally among all participants
- **PERCENTAGE** - Split expense by percentage allocation
- **ITEMIZED** - Assign specific receipt items to specific users
- **EXACT_AMOUNT** - Specify exact amounts for each participant

### Transaction Categories
- **EXPENSE** - Regular expense transactions
- **REPAYMENT** - Repayment transactions between users
- **PAYMENT** - Payment transactions

### Group Types
- **TRIP** - Travel/vacation groups
- **HOME** - Household expense groups
- **COUPLE** - Couple expense tracking
- **OTHER** - General purpose groups

### Production Features
-  **Database Transactions** - Atomic operations for data integrity
-  **Structured Logging** - JSON logging with request correlation IDs (zap)
-  **Rate Limiting** - IP-based rate limiting (500 req/min general, 8 req/min for AI endpoints)
-  **Error Handling** - Comprehensive error handling with custom error codes
-  **CORS Support** - Configurable CORS middleware
-  **Request Timeouts** - 60-second request timeout protection
-  **Graceful Shutdown** - Proper server shutdown handling
-  **Health Checks** - Health check endpoint for monitoring

##  Tech Stack

- **Language:** Go 1.21+
- **Framework:** Chi (lightweight, idiomatic HTTP router)
- **Database:** PostgreSQL (Supabase/Neon compatible)
- **ORM/Query:** pgx/v5 (native PostgreSQL driver)
- **Auth:** Supabase JWT token validation (ES256/HS256)
- **AI:** Google Gemini API (Vision & Pro models)
- **Storage:** Supabase Storage (receipt images, avatars)
- **Logging:** zap (structured JSON logging)
- **Testing:** Go testing framework with mocks

##  Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go              # Application entry point, server setup
├── config/
│   └── config.go               # Configuration management (env vars)
├── database/
│   └── database.go             # Database connection pool & transaction management
├── errors/
│   └── errors.go               # Custom error types and error handling
├── handlers/
│   ├── handlers.go             # Base handlers, error handling utilities
│   ├── group_handlers.go       # Group management endpoints
│   ├── expense_handlers.go     # Expense CRUD endpoints
│   ├── receipt_handlers.go     # Receipt scanning endpoint
│   ├── explanation_handlers.go # AI expense explanation endpoint
│   ├── dashboard_handlers.go  # Dashboard endpoint
│   ├── friend_handlers.go     # Friends management endpoints
│   ├── comment_handlers.go    # Comments & reactions endpoints
│   ├── user_handlers.go       # User profile endpoints
│   ├── avatar_handlers.go     # Avatar upload endpoints
│   └── import_handlers.go     # CSV import endpoints
├── middleware/
│   ├── auth.go                # JWT authentication middleware
│   └── logger.go              # Request logging middleware
├── migrations/
│   ├── 001_initial_schema.up.sql
│   ├── 002_add_category_to_expenses.up.sql
│   ├── 003_add_group_type.up.sql
│   ├── 004_add_expense_payers.up.sql
│   ├── 005_add_dashboard_indexes.up.sql
│   ├── 006_fix_category_constraint.up.sql
│   ├── 007_add_avatar_url_to_groups.up.sql
│   ├── 008_add_tax_fields_to_expenses.up.sql
│   ├── 009_add_explanation_to_expenses.up.sql
│   ├── 010_create_friends_table.up.sql
│   ├── 011_add_date_to_expenses.up.sql
│   ├── 012_fix_date_timezone_shift.up.sql
│   ├── 013_add_placeholder_users.up.sql
│   ├── 014_add_expense_comments.up.sql
│   └── 015_add_placeholder_claiming.up.sql
├── models/
│   └── models.go              # Data models and types
├── repository/
│   ├── user_repository.go     # User data access layer
│   ├── group_repository.go    # Group data access layer
│   ├── expense_repository.go  # Expense data access layer
│   ├── friend_repository.go   # Friend data access layer
│   └── comment_repository.go  # Comment data access layer
├── services/
│   ├── user_service.go        # User business logic
│   ├── group_service.go       # Group business logic
│   ├── expense_service.go     # Expense business logic
│   ├── settlement_service.go  # Settlement calculation logic
│   ├── receipt_service.go     # Receipt parsing service
│   ├── explanation_service.go # AI explanation service
│   ├── dashboard_service.go   # Dashboard aggregation service
│   ├── friend_service.go      # Friend management service
│   ├── comment_service.go     # Comment management service
│   └── import_service.go      # CSV import service
├── storage/
│   ├── storage.go             # Storage interface abstraction
│   └── http_client.go         # Supabase Storage HTTP client
├── scripts/
│   ├── generate_token/        # JWT token generation utility
│   ├── seed/                  # Database seeding script
│   └── show_users/            # User listing utility
├── postman/
│   └── unwise_financial_tests.postman_collection.json  # API test collection
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

##  Architecture

The project follows **Clean Architecture** principles with clear separation of concerns:

### Layer Structure

1. **Handlers Layer** (`handlers/`)
   - HTTP request/response handling
   - Input validation
   - Error response formatting
   - Route registration

2. **Services Layer** (`services/`)
   - Business logic implementation
   - Transaction orchestration
   - Authorization checks
   - Data aggregation and transformation

3. **Repository Layer** (`repository/`)
   - Database query execution
   - Data access abstraction
   - Transaction support via `WithTx()` pattern
   - Batch query optimization

4. **Models Layer** (`models/`)
   - Domain models and DTOs
   - Type definitions and constants
   - JSON serialization tags

5. **Middleware Layer** (`middleware/`)
   - Authentication (JWT validation)
   - Request logging
   - CORS handling
   - Rate limiting

### Key Design Patterns

- **Dependency Injection** - Services and repositories injected via constructors
- **Repository Pattern** - Data access abstraction for testability
- **Transaction Pattern** - `WithTx()` for atomic operations
- **Error Wrapping** - Custom error types with HTTP status mapping
- **Interface Segregation** - Small, focused interfaces per service

##  Setup

### Prerequisites

- Go 1.21 or higher
- PostgreSQL database (Supabase or Neon recommended)
- Google Gemini API key
- Supabase project (for auth and storage)

### Installation

1. **Clone the repository:**
```bash
git clone <repository-url>
cd unwise-backend
```

2. **Install dependencies:**
```bash
go mod download
```

3. **Set up environment variables:**

Create a `.env` file in the root directory:

```env
# Server Configuration
PORT=8080
ENV=development
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
MAX_BODY_SIZE=1048576

# Database
DATABASE_URL=postgresql://user:password@host:5432/dbname

# Supabase Configuration
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_JWT_SECRET=your-jwt-secret
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key

# Storage Configuration
SUPABASE_STORAGE_BUCKET=receipts
SUPABASE_STORAGE_URL=https://your-project.supabase.co/storage/v1
SUPABASE_GROUP_PHOTOS_BUCKET=group-photos
SUPABASE_USER_AVATARS_BUCKET=user-avatars

# AI Services
GEMINI_API_KEY=your-gemini-api-key
```

4. **Run database migrations:**
```bash
# Install migrate tool if not already installed
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run all migrations
make migrate-up

# Or manually
migrate -path migrations -database "$DATABASE_URL" up
```

5. **Seed the database (optional):**
```bash
make seed
```

This creates 6 test users in both Supabase Auth and your database:
- alice@example.com
- bob@example.com
- charlie@example.com
- diana@example.com
- eve@example.com
- frank@example.com

**All test users have password: `TestPassword123!`**

6. **Start the server:**
```bash
make run
# or
go run cmd/server/main.go
```

The server will start on port 8080 (or the port specified in your `.env` file).

##  API Endpoints

### Authentication

All endpoints except `/health` require a Bearer token in the Authorization header:
```
Authorization: Bearer <jwt-token>
```

### Health Check
- `GET /health` - Health check endpoint

### Dashboard
- `GET /api/dashboard` - Get user dashboard with metrics, groups, and recent activity

### User Management
- `GET /api/user/me` - Get current user profile
- `POST /api/user/avatar` - Upload user avatar
- `DELETE /api/user/me` - Delete user account (requires zero balance)
- `GET /api/user/placeholders` - Get claimable placeholder users
- `POST /api/user/placeholders/{placeholderID}/claim` - Claim a placeholder as yourself
- `POST /api/user/placeholders/{placeholderID}/assign` - Assign placeholder to existing user

### Groups

#### Group CRUD
- `GET /api/groups` - Get all groups for authenticated user (with balances)
- `POST /api/groups` - Create a new group
  ```json
  {
    "name": "Trip to Japan",
    "type": "TRIP",
    "member_emails": ["alice@example.com", "bob@example.com"]
  }
  ```
- `GET /api/groups/{groupID}` - Get specific group details
- `PUT /api/groups/{groupID}` - Update group name
- `DELETE /api/groups/{groupID}` - Delete group (requires zero balances)

#### Group Members
- `POST /api/groups/{groupID}/members` - Add member by email
- `POST /api/groups/{groupID}/placeholders` - Add placeholder member
- `DELETE /api/groups/{groupID}/members/{userID}` - Remove member (requires zero balance)

#### Group Data
- `GET /api/groups/{groupID}/expenses` - Get all expenses in group
- `GET /api/groups/{groupID}/transactions` - Get all transactions (expenses + settlements)
- `GET /api/groups/{groupID}/balances` - Get balance edge list (who owes whom)
- `GET /api/groups/{groupID}/settlements` - Get settlement suggestions
- `GET /api/groups/{groupID}/export` - Export group transactions as CSV
- `POST /api/groups/{groupID}/avatar` - Upload group avatar

#### Settlements
- `POST /api/groups/{groupID}/settle` - Create a settlement transaction
  ```json
  {
    "payer_id": "user-id-1",
    "receiver_id": "user-id-2",
    "amount": 50.00
  }
  ```

### Expenses

#### Expense CRUD
- `POST /api/expenses` - Create a new expense
  ```json
  {
    "group_id": "group-uuid",
    "total_amount": 100.00,
    "description": "Dinner at restaurant",
    "split_method": "EQUAL",
    "type": "EXPENSE",
    "splits": [
      {"user_id": "user-1", "amount": 50.00},
      {"user_id": "user-2", "amount": 50.00}
    ],
    "payers": [
      {"user_id": "user-1", "amount_paid": 100.00}
    ],
    "date": "2024-01-15T19:30:00Z",
    "tax": 10.00,
    "cgst": 5.00,
    "sgst": 5.00,
    "service_charge": 5.00
  }
  ```
- `GET /api/expenses/{expenseID}` - Get specific expense details
- `PUT /api/expenses/{expenseID}` - Update expense
- `DELETE /api/expenses/{expenseID}` - Delete expense

#### Expense Comments
- `GET /api/expenses/{expenseID}/comments` - Get all comments for expense
- `POST /api/expenses/{expenseID}/comments` - Create a comment
  ```json
  {
    "text": "Great dinner!"
  }
  ```
- `DELETE /api/expenses/{expenseID}/comments/{commentID}` - Delete your own comment

#### Comment Reactions
- `POST /api/expenses/{expenseID}/comments/{commentID}/reactions` - Add emoji reaction
  ```json
  {
    "emoji": "👍"
  }
  ```
- `DELETE /api/expenses/{expenseID}/comments/{commentID}/reactions` - Remove reaction

### Friends
- `GET /api/friends` - Get all friends with cross-group balances
- `GET /api/friends/search` - Search for potential friends by email/name
- `POST /api/friends` - Add a friend
  ```json
  {
    "friend_id": "user-uuid"
  }
  ```
- `DELETE /api/friends/{friendID}` - Remove a friend

### Receipt Scanning
- `POST /api/scan-receipt` - Upload and parse receipt image
  - Content-Type: `multipart/form-data`
  - Field name: `image`
  - Returns: Parsed receipt data with items, tax breakdown, and total
  - Rate limited: 8 requests per minute per IP

### AI Features
- `POST /api/expenses/explain` - Generate AI explanation for expense
  ```json
  {
    "transaction_id": "expense-uuid"
  }
  ```
  - Rate limited: 8 requests per minute per IP

### Import/Export
- `POST /api/groups/{groupID}/import/splitwise/preview` - Preview Splitwise CSV import
  - Content-Type: `multipart/form-data`
  - Field name: `file` (CSV file)
  - Returns: Preview of expenses that will be imported
- `POST /api/groups/{groupID}/import/splitwise` - Import Splitwise CSV
  - Content-Type: `multipart/form-data`
  - Fields:
    - `file`: CSV file
    - `member_mapping`: JSON mapping of Splitwise users to your users
  ```json
  {
    "Splitwise User 1": "user-uuid-1",
    "Splitwise User 2": "user-uuid-2"
  }
  ```

##  Security Features

- **JWT Authentication** - Supabase JWT validation with ES256/HS256 support
- **Rate Limiting** - IP-based rate limiting (500 req/min general, 8 req/min AI endpoints)
- **Security Headers** - X-Content-Type-Options, X-Frame-Options, CSP, Referrer-Policy, X-XSS-Protection
- **HSTS** - Strict-Transport-Security enabled in production for HTTPS enforcement
- **Request Body Size Limit** - 1MB default limit to prevent memory exhaustion attacks
- **CORS Protection** - Configurable CORS middleware with production warnings
- **Request Timeouts** - 60-second timeout to prevent resource exhaustion
- **Input Validation** - Comprehensive validation for all inputs (UUID format, string length limits)
- **Authorization Checks** - Group membership verification for all operations
- **Error Sanitization** - User-friendly error messages without exposing internals

##  Development

### Running Tests
```bash
make test
```

### Building
```bash
make build
```

### Database Migrations
```bash
# Apply all migrations
make migrate-up

# Rollback last migration
make migrate-down

# Create new migration
migrate create -ext sql -dir migrations -seq <migration_name>
```

### Code Quality
```bash
# Format code
make fmt

# Run linter
make lint

# Run all checks
make check
```

##  Database Schema

### Core Tables
- `users` - User accounts and profiles
- `groups` - Expense groups
- `group_members` - Group membership (many-to-many)
- `expenses` - Expense transactions
- `expense_splits` - How expense is split among users
- `expense_payers` - Who paid for the expense
- `receipt_items` - Individual items from receipt scanning
- `receipt_item_assignments` - Item-to-user assignments
- `comments` - Comments on expenses
- `comment_reactions` - Emoji reactions on comments
- `friends` - Friend relationships

### Key Relationships
- Users ↔ Groups: Many-to-many via `group_members`
- Groups → Expenses: One-to-many
- Expenses → Splits: One-to-many
- Expenses → Payers: One-to-many
- Expenses → Comments: One-to-many
- Users ↔ Friends: Many-to-many via `friends` table

##  Transaction Management

All multi step operations use database transactions for atomicity:

- **Expense Creation** - Creates expense, splits, payers, and receipt items atomically
- **Expense Updates** - Updates expense and recreates splits/payers atomically
- **Group Creation** - Creates group and members atomically
- **Settlement Creation** - Creates settlement expense atomically

Transactions are managed via the `database.WithTx()` pattern, ensuring data integrity even on failures.

##  Error Handling

The API uses a comprehensive error handling system:

- **Custom Error Types** - Structured error codes (e.g., `VALIDATION_001`, `NOT_FOUND_004`)
- **HTTP Status Mapping** - Automatic mapping of error types to HTTP status codes
- **User-Friendly Messages** - Clean error messages for clients
- **Structured Logging** - All errors logged with context and request IDs

### Error Response Format
```json
{
  "error": "User-friendly error message",
  "code": "ERROR_CODE",
  "details": "Additional context (optional)"
}
```

##  Production Considerations

### Already Implemented
-  **Database transactions for data integrity
-  **Structured JSON logging with request correlation IDs
-  **Rate limiting to prevent abuse
-  **Request timeouts
-  **Graceful shutdown
-  **CORS configuration
-  **Error handling framework

### Recommended Enhancements
- Pagination for list endpoints (GetExpenses, GetGroups, etc.)
- Row-level security in database queries
- Soft deletes for audit trails
- Enhanced health check with DB connectivity
- Context timeouts per handler

##  Additional Resources

- [Makefile](Makefile) - Common development commands
- [Migrations](migrations/) - Database migration files

