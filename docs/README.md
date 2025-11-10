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

The site will be available at: `https://observiq.github.io/blitz/`

