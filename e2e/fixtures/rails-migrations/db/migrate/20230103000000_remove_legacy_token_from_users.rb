class RemoveLegacyTokenFromUsers < ActiveRecord::Migration[7.0]
  def change
    remove_column "users", "legacy_token"
  end
end
