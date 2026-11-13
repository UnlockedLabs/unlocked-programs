-- +goose Up
-- 00003 settled on the misspelt 'Incomplete: Transfered'. The status is now
-- spelt 'Incomplete: Transferred' everywhere, so this renames the stored rows.
UPDATE programs.program_class_enrollments
SET enrollment_status = 'Incomplete: Transferred'
WHERE enrollment_status = 'Incomplete: Transfered';

-- +goose Down
-- Nothing to undo: the rows it changed can't be told apart from the ones that
-- were already 'Incomplete: Transferred'.
