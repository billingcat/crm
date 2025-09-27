# billingcat


<img src="https://billingcat.de/images/billingcat.png" width="256">

billingcat is a lightweight CRM and invoicing tool, made for freelancers and small businesses.
It helps you manage contacts, companies and notes – but its main focus is on **invoicing** in a b2b environment.
You can even generate **ZUGFeRD-compliant invoices** (both XML and PDF).

Check out [billingcat.de](https://billingcat.de) – you can sign up for the newsletter there to get notified once the public alpha SaaS goes live!

---

## Features

- Create and download **ZUGFeRD invoices** (XML + PDF)
- Simple but complete **CRM** for contacts, companies and notes
- Focused on **invoice management**
- Designed for freelancers and small businesses
- Open Source and SaaS

>  Note: billingcat is currently available **only in German** (source code and user interface).
> "billingcat" is a registered trademark of Patrick Gundlach.

---

## Tech stack

- [Go](https://go.dev/) + [Echo](https://echo.labstack.com/) for the backend
- [GORM](https://gorm.io/) as ORM
- [Tailwind CSS](https://tailwindcss.com/) + [Alpine.js](https://alpinejs.dev/) for the frontend
- [SQLite](https://www.sqlite.org/) as the default database (built-in, file-based, zero-config)
- [PostgreSQL](https://www.postgresql.org/) as an alternative database
- [speedata Publisher](https://github.com/speedata/publisher) for PDF generation

---

## Getting started

Clone the repo and run:

```bash
go run main.go
```

More detailed setup instructions will follow soon™.

---

## License

billingcat is dual-licensed:

- **Open Source**: released under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.
  That means if you modify and use billingcat as a service, you need to publish your changes.
  See the [LICENSE](./License.md) file for details.

- **Commercial License**: if you want to use billingcat **without the AGPL requirements** (e.g. inside proprietary products or SaaS offerings), get in touch.
Contact: [hallo@billingcat.de]

---

## Trademarks & Logos

The **billingcat** name is a registered trademark of
Patrick Gundlach. The name and the billingcat logo are not part of the open-source license.
Forks or self-hosted instances should replace them with their own branding.

---

## Contributing

Pull requests, bug reports and ideas are always welcome!
Just open an issue or a PR here on GitHub.

---

## Third-Party Licenses

This project makes use of third-party libraries like Alpine.js, Tailwind CSS and Font Awesome (see [Notice.md](./Notice.md)).

---



Made with ❤️ by a small business, for small businesses.