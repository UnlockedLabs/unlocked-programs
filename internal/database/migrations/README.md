# Programs migrations

Goose `.sql` migrations, embedded in the binary and run by `app migrate`. They
run after core's, and their versions are tracked in
`public.goose_db_version_programs`, the ledger core used while Programs was
its module.

Never edit a migration that has been merged; add a new one.

Create new tables as `programs.<name>`. Migrations run as core's database
user, so an unqualified table lands in `public`, where the `programs` role
can't use it.
