UPDATE roles
SET description = 'Every permission, including ones added later'
WHERE name = 'Administrator' AND description = 'Semua izin, termasuk yang ditambahkan kemudian';
