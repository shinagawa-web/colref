# Synthetic pattern battery for Article#title field reference detection.
# Some patterns span multiple lines (e.g. multi-line chain).
# Wrapped in `if false` so this file is safe to load without side effects.

if false
  article = nil
  value   = nil

  # ── Attribute access — read ─────────────────────────────────────────────────
  x = article.title                                     # direct access
  x = Article.find(1).title                             # chained call
  x = Article.where(status: 'published')
             .first
             .title                                     # multi-line chain
  x = "#{article.title}"                                # string interpolation
  x = article&.title                                    # safe navigation

  # ── Attribute access — write ─────────────────────────────────────────────────
  article.title = value                                 # setter

  # ── Hash / symbol access ─────────────────────────────────────────────────────
  x = article[:title]                                   # symbol subscript
  x = article.read_attribute(:title)                    # read_attribute
  article.write_attribute(:title, value)                # write_attribute
  x = article.send(:title)                              # send with symbol
  x = article.public_send(:title)                       # public_send

  # ── ActiveRecord — creation ───────────────────────────────────────────────────
  a = Article.new(title: value)                         # new
  a = Article.create(title: value)                      # create
  a = Article.find_or_create_by(title: value)           # find_or_create_by
  a = Article.find_or_initialize_by(title: value)       # find_or_initialize_by

  # ── ActiveRecord — instance update ───────────────────────────────────────────
  article.update(title: value)                          # update
  article.assign_attributes(title: value)               # assign_attributes
  article.update_column(:title, value)                  # update_column symbol
  article.update_columns(title: value)                  # update_columns hash

  # ── ActiveRecord — query methods ─────────────────────────────────────────────
  Article.where(title: value)                           # where hash
  Article.where("title = ?", value)                     # where string
  Article.where.not(title: value)                       # where.not
  Article.find_by(title: value)                         # find_by
  Article.exists?(title: value)                         # exists?
  Article.order(:title)                                 # order symbol
  Article.order(title: :desc)                           # order hash
  Article.order("title ASC")                            # order string
  Article.pluck(:title)                                 # pluck symbol
  Article.pluck("title")                                # pluck string
  Article.select(:title)                                # select symbol
  Article.select("title, slug")                         # select string
  Article.group(:title)                                 # group symbol
  Article.pick(:title)                                  # pick
  Article.reorder(:title)                               # reorder
  Article.update_all(title: value)                      # update_all

  # ── ActiveRecord — aggregation ────────────────────────────────────────────────
  Article.minimum(:title)                               # minimum
  Article.maximum(:title)                               # maximum
  Article.sum(:title)                                   # sum

  # ── Arel ──────────────────────────────────────────────────────────────────────
  t = Article.arel_table[:title]                        # table subscript
  Article.arel_table[:title].eq(value)                  # arel condition

  # ── Model declarations ────────────────────────────────────────────────────────
  # (see app/models/article.rb for validates and scope patterns)

  # ── Serialization / presentation ─────────────────────────────────────────────
  params.require(:article).permit(:title, :slug)        # strong params permit
  # AMS attributes: see app/serializers/article_serializer.rb

  # ── Dynamic / metaprogramming ─────────────────────────────────────────────────
  article.respond_to?(:title)                           # respond_to?
  article.instance_variable_get(:@title)               # instance_variable_get (not detectable)
  article.title_changed?                                # attribute_changed? (name-mangled)
  Article.find_by_title(value)                          # dynamic finder (name-mangled)
end
