# `database`

Shared PostgreSQL helpers.

Current scope:

- load PostgreSQL config from env
- build pgx connection string
- create and verify a shared `pgxpool.Pool`
- load ordered `.sql` migrations from disk
- apply unapplied migrations and track them in `schema_migrations`
