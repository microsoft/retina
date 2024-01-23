# Website

This website is built using [Docusaurus 2](https://docusaurus.io/), a modern static website generator.

## Development

When adding a new doc, make sure to add it to /site/sidebars.js

To test, run `make docs` to spin up local webserver and view changes with hot reload

## Using Yarn

### Installation

Install `yarn` (e.g. for Ubuntu, try [this guide](https://www.linuxcapable.com/how-to-install-yarn-on-ubuntu-linux/#install-yarn-on-ubuntu-2204-or-2004-via-nodesource)).

In this directory (*retina/site/*), run:
```bash
yarn
```

### Local Development

```bash
yarn start
```

This command starts a local development server and opens up a browser window. Most changes are reflected live without having to restart the server.

### Build

```bash
yarn build
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.

## Deployment

Using SSH:

```bash
USE_SSH=true yarn deploy
```

Not using SSH:

```bash
GIT_USER=<Your GitHub username> yarn deploy
```

If you are using GitHub pages for hosting, this command is a convenient way to build the website and push to the `gh-pages` branch.
