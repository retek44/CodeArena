-- Modify "problems" table
ALTER TABLE "problems" ADD COLUMN "checker_tolerance" double precision NULL, ADD COLUMN "checker_source" text NULL;
