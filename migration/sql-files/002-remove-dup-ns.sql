-- Remove duplicate registered namespaces
-- Keep the oldest one
DELETE FROM namespaces 
WHERE id IN (
	SELECT id
	FROM (
		SELECT id, ROW_NUMBER() OVER (partition BY name ORDER BY created_at desc) AS rnum
		FROM namespaces
	) t
	WHERE t.rnum > 1
)
