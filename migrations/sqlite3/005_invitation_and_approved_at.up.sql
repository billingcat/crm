CREATE TABLE invitations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    email TEXT,
    expires_at DATETIME,
    created_at DATETIME NOT NULL
);
