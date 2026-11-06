-- +goose Up
-- A resident's transfer wrote 'Incomplete: Transferred', but everything else,
-- the class outcomes, the reports filter and the frontend, reads
-- 'Incomplete: Transfered', so transferred enrollments were left out of them.
-- The transfer now writes 'Incomplete: Transfered' too, and this gives the
-- enrollments it already ended the same status.
UPDATE programs.program_class_enrollments
SET enrollment_status = 'Incomplete: Transfered'
WHERE enrollment_status = 'Incomplete: Transferred';

-- +goose Down
-- Nothing to undo: the rows it changed can't be told apart from the ones that
-- were already 'Incomplete: Transfered', and the other spelling was the bug.
