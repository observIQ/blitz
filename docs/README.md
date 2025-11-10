# Blitz Documentation

This directory contains the documentation for Blitz, which is automatically built and deployed to GitHub Pages.

## Local Development

To preview the documentation locally using Jekyll:

```bash
cd docs
bundle install
bundle exec jekyll serve
```

Then open [http://localhost:4000](http://localhost:4000) in your browser.

## GitHub Pages Setup

The documentation is automatically deployed to GitHub Pages via the `.github/workflows/pages.yml` workflow.

To enable GitHub Pages:

1. Go to your repository settings on GitHub
2. Navigate to "Pages" in the left sidebar
3. Under "Source", select "GitHub Actions"
4. The site will be automatically built and deployed when changes are pushed to the `main` branch

The production site will be available at: `https://observiq.github.io/blitz/`

## Testing on Branches

You can test documentation changes on branches before merging to `main`:

### Pull Request Previews

When you open a pull request that modifies documentation files:
- The workflow automatically builds and deploys a preview
- A preview URL will be posted as a comment on the PR
- The preview is available for review before merging

### Branch Testing

When you push changes to any branch (not just `main`):
- The workflow builds the site
- For non-main branches, it deploys to a preview environment
- You can view the preview URL in the workflow run summary

This allows you to:
- Test documentation changes in isolation
- Share preview links with reviewers
- Verify links and formatting before merging

