package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Column represents a column in a datasource
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Datasource represents a Tinybird datasource
type Datasource struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

// Schema holds all datasources and their columns
type Schema struct {
	Datasources []Datasource
}

// FetchSchema fetches the schema from Tinybird API
func (c *TinybirdClient) FetchSchema() (*Schema, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v0/datasources", c.host), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch datasources: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tinybird error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Datasources []struct {
			Name    string `json:"name"`
			Columns []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"columns"`
		} `json:"datasources"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	schema := &Schema{}
	for _, ds := range result.Datasources {
		datasource := Datasource{Name: ds.Name}
		for _, col := range ds.Columns {
			datasource.Columns = append(datasource.Columns, Column{
				Name: col.Name,
				Type: col.Type,
			})
		}
		schema.Datasources = append(schema.Datasources, datasource)
	}

	return schema, nil
}

// GenerateGrammar creates a Lark grammar from the schema
func (s *Schema) GenerateGrammar() string {
	var sb strings.Builder

	// Header
	sb.WriteString(`// Auto-generated ClickHouse SQL grammar
// ---------- Whitespace ----------
SP: " "

// ---------- Punctuation ----------
COMMA: ","
SEMI: ";"
LPAREN: "("
RPAREN: ")"

// ---------- Operators ----------
GT: ">"
LT: "<"
GTE: ">="
LTE: "<="
EQ: "="
NEQ: "!="

// ---------- Start ----------
start: select_stmt SEMI

// ---------- SELECT statement ----------
select_stmt: "SELECT" SP select_list SP "FROM" SP table (SP where_clause)? (SP group_clause)? (SP order_clause)? (SP limit_clause)?

// ---------- Select list ----------
select_list: select_item (COMMA SP select_item)*
select_item: agg_expr | column | star
star: "*"

// ---------- Aggregation ----------
agg_expr: agg_func LPAREN agg_arg RPAREN (SP "AS" SP alias)?
agg_func: "SUM" | "COUNT" | "AVG" | "MIN" | "MAX"
agg_arg: column | star
alias: IDENTIFIER

`)

	// Generate table rule
	sb.WriteString("// ---------- Tables ----------\n")
	if len(s.Datasources) > 0 {
		var tableNames []string
		for _, ds := range s.Datasources {
			tableNames = append(tableNames, fmt.Sprintf(`"%s"`, ds.Name))
		}
		sb.WriteString(fmt.Sprintf("table: %s\n\n", strings.Join(tableNames, " | ")))
	} else {
		sb.WriteString("table: IDENTIFIER\n\n")
	}

	// Collect all unique columns across all tables
	columnSet := make(map[string]bool)
	for _, ds := range s.Datasources {
		for _, col := range ds.Columns {
			columnSet[col.Name] = true
		}
	}

	// Generate column rules
	sb.WriteString("// ---------- Columns ----------\n")
	if len(columnSet) > 0 {
		var colRules []string
		i := 0
		for colName := range columnSet {
			ruleName := fmt.Sprintf("COL_%d", i)
			sb.WriteString(fmt.Sprintf("%s: \"%s\"\n", ruleName, colName))
			colRules = append(colRules, ruleName)
			i++
		}
		sb.WriteString(fmt.Sprintf("column: %s\n\n", strings.Join(colRules, " | ")))
	} else {
		sb.WriteString("column: IDENTIFIER\n\n")
	}

	// WHERE clause
	sb.WriteString(`// ---------- WHERE clause ----------
where_clause: "WHERE" SP condition (SP "AND" SP condition)*
condition: column SP compare_op SP value
compare_op: GTE | LTE | GT | LT | EQ | NEQ
value: STRING | NUMBER | DATETIME

// ---------- GROUP BY ----------
group_clause: "GROUP" SP "BY" SP column (COMMA SP column)*

// ---------- ORDER BY ----------
order_clause: "ORDER" SP "BY" SP sort_item (COMMA SP sort_item)*
sort_item: column (SP sort_dir)?
sort_dir: "ASC" | "DESC"

// ---------- LIMIT ----------
limit_clause: "LIMIT" SP NUMBER

// ---------- Terminals ----------
IDENTIFIER: /[A-Za-z_][A-Za-z0-9_]*/
NUMBER: /[0-9]+(\.[0-9]+)?/
STRING: /'[^']*'/
DATETIME: /'[0-9]{4}-[0-9]{2}-[0-9]{2}( [0-9]{2}:[0-9]{2}:[0-9]{2})?'/
`)

	return sb.String()
}

// GenerateToolDescription creates a description of available tables and columns
func (s *Schema) GenerateToolDescription() string {
	var sb strings.Builder

	sb.WriteString("Generates valid ClickHouse SQL queries.\n\n")
	sb.WriteString("Available tables and columns:\n")

	for _, ds := range s.Datasources {
		sb.WriteString(fmt.Sprintf("\n## %s\n", ds.Name))
		for _, col := range ds.Columns {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", col.Name, col.Type))
		}
	}

	sb.WriteString("\nSupported operations:\n")
	sb.WriteString("- SELECT with columns or aggregates (SUM, COUNT, AVG, MIN, MAX)\n")
	sb.WriteString("- WHERE with comparisons (=, !=, >, <, >=, <=)\n")
	sb.WriteString("- GROUP BY columns\n")
	sb.WriteString("- ORDER BY columns (ASC/DESC)\n")
	sb.WriteString("- LIMIT\n\n")
	sb.WriteString("YOU MUST generate syntactically valid SQL that conforms to the grammar.")

	return sb.String()
}

