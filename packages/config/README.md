# `config`

Shared environment parsing helpers.

Current helpers:

- `GetString`
- `MustString`
- `GetInt`
- `GetBool`
- `GetDuration`

Behavior:

- on first config access, the package searches upward from the current working directory
- if found, it loads `.env` and then `.env.local`
- existing process environment variables are not overridden
