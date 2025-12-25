package main

// ClickHouse SQL CFG grammar for the order_items table
// Columns: order_id, order_item_id, product_id, seller_id, shipping_limit_date, price, freight_value
const ClickHouseGrammar = `
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

// ---------- Select list (columns or aggregates) ----------
select_list: select_item (COMMA SP select_item)*
select_item: agg_expr | column | star
star: "*"

// ---------- Aggregation ----------
agg_expr: agg_func LPAREN agg_arg RPAREN (SP "AS" SP alias)?
agg_func: "SUM" | "COUNT" | "AVG" | "MIN" | "MAX"
agg_arg: column | star
alias: IDENTIFIER

// ---------- Columns (restricted to our schema) ----------
column: COL_ORDER_ID | COL_ORDER_ITEM_ID | COL_PRODUCT_ID | COL_SELLER_ID | COL_SHIPPING_DATE | COL_PRICE | COL_FREIGHT
COL_ORDER_ID: "order_id"
COL_ORDER_ITEM_ID: "order_item_id"
COL_PRODUCT_ID: "product_id"
COL_SELLER_ID: "seller_id"
COL_SHIPPING_DATE: "shipping_limit_date"
COL_PRICE: "price"
COL_FREIGHT: "freight_value"

// ---------- Table ----------
table: "order_items"

// ---------- WHERE clause ----------
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
`

const ToolDescription = `Generates valid ClickHouse SQL queries for the order_items table.

Available columns:
- order_id (String): Unique order identifier
- order_item_id (Int32): Item number within order  
- product_id (String): Product identifier
- seller_id (String): Seller identifier
- shipping_limit_date (DateTime): Shipping deadline
- price (Float64): Item price
- freight_value (Float64): Shipping cost

Supported operations:
- SELECT with columns or aggregates (SUM, COUNT, AVG, MIN, MAX)
- WHERE with comparisons (=, !=, >, <, >=, <=)
- GROUP BY columns
- ORDER BY columns (ASC/DESC)
- LIMIT

YOU MUST generate syntactically valid SQL that conforms to the grammar. Think carefully about the query structure.`

