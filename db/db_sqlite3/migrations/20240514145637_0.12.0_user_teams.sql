-- +goose Up
CREATE TABLE team_users (
    team_id INTEGER,
    user_id INTEGER,
    role_id INTEGER,
    FOREIGN KEY (role_id) REFERENCES roles(id),
    PRIMARY KEY (team_id, user_id)
);

INSERT INTO "roles" ("slug", "name", "description") VALUES ("contributor", "Contributor", "Team Contributor");
INSERT INTO "roles" ("slug", "name", "description") VALUES ("viewer", "Viewer", "Team Viewer");
INSERT INTO "roles" ("slug", "name", "description") VALUES ("team_admin", "RoleTeamAdmin ", "Team Admin");


INSERT INTO "permissions" ("slug", "name", "description") VALUES ("view_team_objects", "View Team Objects", "view team objects in Gophish");
INSERT INTO "permissions" ("slug", "name", "description") VALUES ("modify_team_objects", "Edit Team Objects", "edit team objects in Gophish");
INSERT INTO "permissions" ("slug", "name", "description") VALUES ("delete_team_objects", "Delete Team Objects", "delete team objects in Gophish");

INSERT INTO "role_permissions" ("role_id", "permission_id")
SELECT r.id, p.id FROM roles AS r, "permissions" AS p
WHERE r.id IN (SELECT "id" FROM roles WHERE "slug"="contributor")
AND p.id=(SELECT "id" FROM "permissions" WHERE "slug"="modify_team_objects");	

INSERT INTO "role_permissions" ("role_id", "permission_id")
SELECT r.id, p.id FROM roles AS r, "permissions" AS p
WHERE r.id IN (SELECT "id" FROM roles WHERE "slug"="viewer")
AND p.id=(SELECT "id" FROM "permissions" WHERE "slug"="view_team_objects");	

INSERT INTO "role_permissions" ("role_id", "permission_id")
SELECT r.id, p.id FROM roles AS r, "permissions" AS p
WHERE r.id IN (SELECT "id" FROM roles WHERE "slug"="team_admin")
AND p.id=(SELECT "id" FROM "permissions" WHERE "slug"="delete_team_objects");	

INSERT INTO "role_permissions" ("role_id", "permission_id")
SELECT r.id, p.id FROM roles AS r, "permissions" AS p
WHERE r.id IN (SELECT "id" FROM roles WHERE "slug"="team_admin")
AND p.id=(SELECT "id" FROM "permissions" WHERE "slug"="modify_team_objects");	

-- +goose Down
DROP TABLE team_users;
