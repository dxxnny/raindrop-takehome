# NL → SQL Query Generator

Converts natural language to valid ClickHouse SQL using GPT-5's CFG constrained generation.

## How It Works

1. User asks a question in plain English
2. GPT-5 generates SQL constrained by a Lark grammar
3. Query executes against Tinybird (ClickHouse)
4. Results displayed in the UI

## Project Structure

```
api/
  query/index.go       # POST /api/query - NL to SQL
  eval/index.go        # GET /api/eval - Run test suite
cmd/
  eval-check/main.go   # Build-time eval gate
pkg/shared/
  openai.go            # GPT-5 client with CFG
  tinybird.go          # ClickHouse execution
  schema.go            # Dynamic grammar from DB schema
  eval.go              # Automated test cases
  config.go            # Environment config
public/                # Static frontend
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | GPT-5 API key |
| `TINYBIRD_HOST` | e.g., `https://api.us-west-2.aws.tinybird.co` |
| `TINYBIRD_TOKEN` | Tinybird read token |

*Automated evals run at build-time and will fail the deployment if any test fails.*