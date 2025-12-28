# BreathAIPay
A bridge connecting Stripe and OpenWebUI - automating payments & adding credits

***
### 语言/Language
| [简体中文](README.md) | English |
***

### Project Startup
```bash
go run main.go
```
The project listens on port 4399


### Environment Variables
Environment variables can be stored in the `.env` file in the project root directory or written as system environment variables. The program will prioritize looking for variables in `.env`.  
> An asterisk (*) indicates a required field, <del>strikethrough</del> indicates deprecated.

| Variable Name | Purpose |
| :--: | :--: |
| DEBUG_MODE | Sets whether it's debug mode (anything other than true is not debug) |
| SMTP_HOST | SMTP protocol host |
| SMTP_USERNAME | SMTP login name |
| SMTP_PORT | SMTP Login Port |
| SMTP_PASSWORD | SMTP Login Password |
| OPENWEBUI_INTERNATIONAL_TOKEN * | International Site JWT Token |
| <del>OPENWEBUI_CHINESE_TOKEN</del> | <del>Chinese Site JWT Token</del> |
| STRIPE_PRIVATE_KEY * | Stripe private key |
| STRIPE_PUBLIC_KEY * | Stripe public key |
| TRUST_ALL_PROXIES | Whether to trust all reverse proxies (default is false). Enabling this option is an unwise decision. |
> Warning: If Stripe public/private keys are not configured, the program will not start

### Product List
> Located on line 40 of `main.go`

```go
[]Product{
{ID: 1, Name: "100,000 Points", Price: 20.0, Points: 100000},
    {ID: 2, Name: "500,000 Points", Price: 50.0, Points: 500000},
    {ID: 3, Name: "1,000,000 Points", Price: 100.0, Points: 1000000},
}
```
#### Configuration Items
| Configuration Item | Description |
| :--: | :--: |
| ID | A unique number used for front-end and back-end communication |
| Name | The product name displayed to users, with no impact on price or subsequent points |
| Price | Unit price of the product |
| Points | Points added with each purchase |
> Note: The handling fee is automatically calculated by the program based on the Price  
> Note: Price is in CNY

### Database Instructions
The project uses a Sqlite database, which will **automatically** create `customers.db` and `orders.db` in the root directory. Please ensure the program has sufficient write permissions

### Additional Notes
- The project does not rely on static CDN services, but instead uses local server-hosted JS/CSS files