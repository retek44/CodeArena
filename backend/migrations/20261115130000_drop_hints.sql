-- Contract step of ADR-0021: no released API version reads or writes hints any more.
-- atlas:nolint destructive
DROP TABLE "hints";
