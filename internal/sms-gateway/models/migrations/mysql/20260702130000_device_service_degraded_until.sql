-- +goose Up
-- +goose StatementBegin
ALTER TABLE `devices`
ADD COLUMN IF NOT EXISTS `service_degraded_until` datetime(3) NULL DEFAULT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE `devices` DROP COLUMN IF EXISTS `service_degraded_until`;
-- +goose StatementEnd
