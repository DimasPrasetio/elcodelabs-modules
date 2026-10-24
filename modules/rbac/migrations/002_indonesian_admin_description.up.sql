UPDATE roles
SET description = 'Semua izin, termasuk yang ditambahkan kemudian'
WHERE name = 'Administrator' AND description = 'Every permission, including ones added later';
