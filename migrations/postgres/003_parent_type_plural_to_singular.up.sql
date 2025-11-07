
-- Notes: companies -> company, people -> person
UPDATE notes SET parent_type = 'company' WHERE parent_type = 'companies';
UPDATE notes SET parent_type = 'person'  WHERE parent_type = 'people';

-- ContactInfos: companies -> company, people -> person
UPDATE contact_infos SET parent_type = 'company' WHERE parent_type = 'companies';
UPDATE contact_infos SET parent_type = 'person'  WHERE parent_type = 'people';
