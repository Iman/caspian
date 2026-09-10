# Wiki navigation

The published wiki has a separate Git repository at `https://github.com/Iman/caspian.wiki.git`.
Keep its Markdown files in sync with `docs/wiki/` in this repository.

GitHub uses one `_Sidebar.md` for the entire wiki.
Each language has a collapsible sidebar section with links to its home page and common tasks.
Do not create translated `_Sidebar` files: GitHub does not select them by the reader's language.

Each home page lists the full set of guides in three groups: setup, use and maintenance, and development.
Each guide starts with links to the same topic in seven languages, plus local home and troubleshooting links.
Persian, Arabic, and Urdu content uses a right-to-left wrapper. The language switcher uses a separate left-to-right wrapper.
Keep existing topic URLs and explicit heading anchors so saved links continue to work.

## Edit and validate

Navigation labels and home pages come from `scripts/wiki_navigation.py`.
After you change these labels, regenerate the home pages and sidebar:

```sh
python scripts/wiki_navigation.py --write
```

If you change the shared page navigation, update that block in every guide too.
Keep technical commands, warnings, source links, and examples intact.
Translation hashes record the English source used for review. Update them only after reviewing the corresponding changes.
Automated parity checks do not establish translation quality; the translated guide text still needs independent native-speaker review.

Run the wiki checks before publishing:

```sh
python scripts/check-wiki.py
python scripts/wiki_navigation.py --check
python scripts/test_wiki_navigation.py
git diff --check
```

## Publish

1. Fetch the wiki repository and compare it with the local source before copying files.
2. Copy the reviewed Markdown changes to the wiki checkout, including explicit removal of obsolete sidebar files.
3. Review the wiki diff and run the checker against that checkout.
4. Commit and push the wiki changes without force.
5. Make sure that the remote commit matches the local commit and inspect the rendered home pages, sidebar, and language links.

The `Caspian-wiki` pages retain their old URLs and show the same directory as the corresponding home pages.

GitHub documents [custom sidebars](https://docs.github.com/en/communities/documenting-your-project-with-wikis/creating-a-footer-or-sidebar-for-your-wiki) and [local wiki editing](https://docs.github.com/en/communities/documenting-your-project-with-wikis/adding-or-editing-wiki-pages).
