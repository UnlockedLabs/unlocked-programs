-- +goose Up
-- change_log_entries is Programs' change log (programs, classes, sessions and
-- enrollments) and no other service uses it, so it moves into the programs
-- schema with the rest of Programs' tables (EN-207).
--
-- SET SCHEMA keeps the rows, indexes, the foreign key to users, and the id
-- sequence, which moves with the table. It keeps the grants too: the programs
-- role may still read, add and delete entries, but never change one. The
-- schema's default grants don't apply, since they're only for new tables.
--
-- The move holds a brief exclusive lock on the table. If a long query holds
-- one first, this gives up instead of queueing every later query behind it;
-- the transaction rolls back and the migration can run again.
SET LOCAL lock_timeout = '5s';

ALTER TABLE public.change_log_entries SET SCHEMA programs;

-- +goose Down
-- Moving it back takes the same lock, so it gives up the same way.
SET LOCAL lock_timeout = '5s';

ALTER TABLE programs.change_log_entries SET SCHEMA public;
