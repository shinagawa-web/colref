class Article < ApplicationRecord
  belongs_to :author

  validates :title, presence: true
  validates :slug, presence: true, uniqueness: true

  scope :titled, ->(t) { where(title: t) }
end
