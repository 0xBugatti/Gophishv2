-- +goose Up

CREATE TABLE "item_teams" ("team_id" integer,"item_id" integer,PRIMARY KEY ("team_id","item_id"));

CREATE TABLE "items" ("id" integer PRIMARY KEY AUTOINCREMENT,"item_type" text,"item_type_id" integer,CONSTRAINT "fk_pages_items" FOREIGN KEY ("item_type_id") REFERENCES "pages"("id"),CONSTRAINT "fk_campaigns_items" FOREIGN KEY ("item_type_id") REFERENCES "campaigns"("id"), CONSTRAINT "fk_templates_items" FOREIGN KEY ("item_type_id") REFERENCES "templates"("id"), CONSTRAINT "fk_smtps_items" FOREIGN KEY ("item_type_id") REFERENCES "smtps"("id"), CONSTRAINT "fk_groups_items" FOREIGN KEY ("item_type_id") REFERENCES "groups"("id"), CONSTRAINT "fk_scenarios_items" FOREIGN KEY ("item_type_id") REFERENCES "scenarios"("id"));

INSERT INTO items (item_type, item_type_id) SELECT id, "campaigns" FROM campaigns;
INSERT INTO items (item_type, item_type_id) SELECT id, "pages" FROM pages;
INSERT INTO items (item_type, item_type_id) SELECT id, "templates" FROM templates;
INSERT INTO items (item_type, item_type_id) SELECT id, "smtp" FROM smtp;
INSERT INTO items (item_type, item_type_id) SELECT id, "groups" FROM groups;

-- +goose Down
DROP TABLE item_teams;
