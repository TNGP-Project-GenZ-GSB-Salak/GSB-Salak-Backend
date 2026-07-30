ALTER TABLE salak.holdings ADD COLUMN ticket_letter varchar(1) NOT NULL DEFAULT 'ก';
ALTER TABLE salak.holdings ALTER COLUMN ticket_letter DROP DEFAULT;
