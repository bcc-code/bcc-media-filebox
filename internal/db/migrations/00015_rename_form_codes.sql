-- +goose Up
-- +goose StatementBegin
UPDATE targets SET form_key = 'masters' WHERE form_key = 'camera_dailies';
-- +goose StatementEnd
-- +goose StatementBegin
-- bcc_media form removed: drop the reference so the target falls back to
-- free upload rather than failing validation with ERR_INVALID_FORM.
UPDATE targets SET form_key = NULL WHERE form_key = 'bcc_media';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE targets SET form_key = 'camera_dailies' WHERE form_key = 'masters';
-- +goose StatementEnd
