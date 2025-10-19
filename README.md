# Google Doc Review AI

this is google doc review ai, it is a tool that helps you review your google docs.

use claude code and cursor to build this tool.

## Usage

### Setup

copy the .env.example to .env and fill in the values

```bash
cp .env.example .env
```

fill in the values in .env

```bash
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
```

### Run

```bash
go run cmd/server/main.go
```
