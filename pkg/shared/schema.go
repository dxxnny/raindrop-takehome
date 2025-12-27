package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
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

// sanitizeColumnName converts a column name to a valid Lark terminal name
func sanitizeColumnName(name string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9_]`)
	sanitized := re.ReplaceAllString(name, "_")
	return "COL_" + strings.ToUpper(sanitized)
}

// GenerateGrammar creates a Lark grammar from the schema
func (s *Schema) GenerateGrammar() string {
	var sb strings.Builder

	sb.WriteString(`# Auto-generated ClickHouse SQL grammar

SP: " "
COMMA: ","
SEMI: ";"
LPAREN: "("
RPAREN: ")"
GT: ">"
LT: "<"
GTE: ">="
LTE: "<="
EQ: "="
NEQ: "!="

start: select_stmt SEMI
select_stmt: "SELECT" SP select_list SP "FROM" SP table (SP where_clause)? (SP group_clause)? (SP order_clause)? (SP limit_clause)?
select_list: select_item (COMMA SP select_item)*
select_item: agg_expr | column | star
star: "*"
agg_expr: agg_func LPAREN agg_arg RPAREN (SP "AS" SP alias)?
agg_func: "SUM" | "COUNT" | "AVG" | "MIN" | "MAX"
agg_arg: column | star
alias: IDENTIFIER

`)

	// Generate table rule
	sb.WriteString("# Tables\n")
	if len(s.Datasources) > 0 {
		tableNames := make([]string, 0, len(s.Datasources))
		for _, ds := range s.Datasources {
			tableNames = append(tableNames, ds.Name)
		}
		sort.Strings(tableNames)

		quotedNames := make([]string, 0, len(tableNames))
		for _, name := range tableNames {
			quotedNames = append(quotedNames, fmt.Sprintf(`"%s"`, name))
		}
		sb.WriteString(fmt.Sprintf("table: %s\n\n", strings.Join(quotedNames, " | ")))
	} else {
		sb.WriteString("table: IDENTIFIER\n\n")
	}

	// Collect all unique columns
	columnSet := make(map[string]bool)
	for _, ds := range s.Datasources {
		for _, col := range ds.Columns {
			columnSet[col.Name] = true
		}
	}

	columnNames := make([]string, 0, len(columnSet))
	for name := range columnSet {
		columnNames = append(columnNames, name)
	}
	sort.Strings(columnNames)

	// Generate column rules
	sb.WriteString("# Columns\n")
	if len(columnNames) > 0 {
		colRules := make([]string, 0, len(columnNames))
		for _, colName := range columnNames {
			ruleName := sanitizeColumnName(colName)
			sb.WriteString(fmt.Sprintf("%s: \"%s\"\n", ruleName, colName))
			colRules = append(colRules, ruleName)
		}
		sb.WriteString(fmt.Sprintf("column: %s\n\n", strings.Join(colRules, " | ")))
	} else {
		sb.WriteString("column: IDENTIFIER\n\n")
	}

	sb.WriteString(`where_clause: "WHERE" SP condition (SP "AND" SP condition)*
condition: column SP compare_op SP value
compare_op: GTE | LTE | GT | LT | EQ | NEQ
value: STRING | NUMBER | DATETIME
group_clause: "GROUP" SP "BY" SP column (COMMA SP column)*
order_clause: "ORDER" SP "BY" SP sort_item (COMMA SP sort_item)*
sort_item: column (SP sort_dir)?
sort_dir: "ASC" | "DESC"
limit_clause: "LIMIT" SP NUMBER
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

	dsNames := make([]string, 0, len(s.Datasources))
	dsMap := make(map[string]Datasource)
	for _, ds := range s.Datasources {
		dsNames = append(dsNames, ds.Name)
		dsMap[ds.Name] = ds
	}
	sort.Strings(dsNames)

	for _, name := range dsNames {
		ds := dsMap[name]
		sb.WriteString(fmt.Sprintf("\n## %s\n", ds.Name))

		colNames := make([]string, 0, len(ds.Columns))
		colMap := make(map[string]Column)
		for _, col := range ds.Columns {
			colNames = append(colNames, col.Name)
			colMap[col.Name] = col
		}
		sort.Strings(colNames)

		for _, colName := range colNames {
			col := colMap[colName]
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

// GenerateUserHint creates a brief, user-friendly summary of available data
func (s *Schema) GenerateUserHint() string {
	if len(s.Datasources) == 0 {
		return "No data available."
	}

	var parts []string
	for _, ds := range s.Datasources {
		colNames := make([]string, 0, len(ds.Columns))
		for _, col := range ds.Columns {
			colNames = append(colNames, col.Name)
		}
		sort.Strings(colNames)
		parts = append(parts, fmt.Sprintf("%s (%s)", ds.Name, strings.Join(colNames, ", ")))
	}
	sort.Strings(parts)

	return "Available data: " + strings.Join(parts, "; ")
}
