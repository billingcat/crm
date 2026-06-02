# billingcat

<img src="https://billingcat.de/images/billingcat.png" width="256">

billingcat is a small CRM and invoicing tool for freelancers and small businesses in a B2B context.
Besides managing contacts, companies and notes, it can generate ZUGFeRD-compliant invoices (XML and PDF).

The project website is [billingcat.de](https://billingcat.de). The hosted version is currently in beta — registration is available on request via hallo@billingcat.de.

Note: billingcat is currently German only — both the source code comments and the user interface.

## Tech stack

- [Go](https://go.dev/) with [Echo](https://echo.labstack.com/)
- [GORM](https://gorm.io/) for database access
- [Tailwind CSS](https://tailwindcss.com/) and [Alpine.js](https://alpinejs.dev/) on the frontend
- SQLite by default, PostgreSQL optional
- [speedata Publisher](https://github.com/speedata/publisher) for PDF generation

## Running it locally

```bash
git clone https://github.com/billingcat/crm.git
cd crm
cp config.toml.example config.toml
go run -tags sqlite .
```

Then open <http://localhost:5555/register>. The first user to register becomes the admin.

For production deployment, PostgreSQL setup, mail configuration and PDF generation, see the manual at <https://docs.billingcat.de/de/docs/install/> (German).

## License

billingcat is dual-licensed.

The open source version is released under the GNU Affero General Public License v3.0 (AGPL-3.0); see [License.md](./License.md). Running a modified version as a service means you have to publish your changes.

If that doesn't work for you — for example inside a proprietary product or a closed-source SaaS — a commercial license is available. Contact hallo@billingcat.de.

## Trademarks

"billingcat" is a registered trademark of Patrick Gundlach. The name and the logo are not covered by the open source license, so forks and self-hosted instances should use their own branding.

## Contributing

Issues and pull requests are welcome.

## Third-party licenses

See [Notice.md](./Notice.md) for the third-party libraries used (Alpine.js, Tailwind CSS, Font Awesome and others).
