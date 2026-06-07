package models

type Schema struct {
	Name   string
	Tables []Table
}

type Table struct {
	Schema     string
	Name       string
	Columns    []Column
	Indexes    []Index
	PrimaryKey []string
}

type Column struct {
	Name            string
	Type            string
	IsNullable      bool
	DefaultValue    *string
	IsAutoIncrement bool
	CharacterSet    *string
	Collation       *string
}

type Index struct {
	Name      string
	Columns   []string
	IsUnique  bool
	IsPrimary bool
}

type DataBatch struct {
	Schema    string
	Table     string
	Columns   []string
	Rows      [][]interface{}
	BatchSize int
}

type TransferProgress struct {
	Schema        string
	Table         string
	TotalRows     int64
	ProcessedRows int64
	IsCompleted   bool
	Error         error
}
