ActiveRecord::Schema[7.0].define(version: 2024_01_01_000000) do
  create_table "users", force: :cascade do |t|
    t.string "email", null: false
    t.string "name"
  end

  create_table "posts", force: :cascade do |t|
    t.string "title", null: false
  end
end
