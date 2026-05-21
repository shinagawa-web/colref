ActiveRecord::Schema[7.0].define(version: 2024_01_01_000000) do
  create_table "authors", force: :cascade do |t|
    t.string "name", null: false
  end

  create_table "articles", force: :cascade do |t|
    t.string "title", null: false
    t.string "slug", null: false
    t.string "email"
    t.string "status"
    t.integer "author_id"
  end
end
