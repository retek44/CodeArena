-- Modify "outbox_messages" table
ALTER TABLE "outbox_messages" ADD COLUMN "trace_parent" character varying NULL;
