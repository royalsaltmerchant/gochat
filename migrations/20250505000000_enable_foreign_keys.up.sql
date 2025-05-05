-- Enable foreign key support
PRAGMA foreign_keys = ON;

-- Verify foreign keys are enabled
SELECT CASE WHEN foreign_keys = 1 THEN 'Foreign keys enabled' ELSE 'Foreign keys disabled' END
FROM pragma_foreign_keys; 