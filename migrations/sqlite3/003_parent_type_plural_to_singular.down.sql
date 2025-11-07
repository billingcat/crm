-- Rollback: company -> companies, person -> people
UPDATE notes SET parent_type = 'companies' WHERE parent_type = 'company';
UPDATE notes SET parent_type = 'people'    WHERE parent_type = 'person';

UPDATE contact_infos SET parent_type = 'companies' WHERE parent_type = 'company';
UPDATE contact_infos SET parent_type = 'people'    WHERE parent_type = 'person';
