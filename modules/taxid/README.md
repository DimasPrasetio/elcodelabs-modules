# taxid

Go kit: Indonesian tax identities. Installed into `apps/api/internal/shared/taxid`; a module that
uses it lists `"go_kits": ["taxid"]`. Standard library only.

It checks **formats**. Whether an NPWP or NIK is registered, or a party is a PKP, only DJP
(Coretax) and Dukcapil can say.

| Function | Does |
|---|---|
| `taxid.Kind` | `npwp`, `nik`, `passport`, `none` — the buyer ID types of Coretax; `Valid()` |
| `NormalizeNPWP(s)` | The 16-digit NPWP: 16 digits as given, or a 15-digit old NPWP with a leading `0` (PMK 112/2022). Dots, dashes, and spaces are ignored; letters are refused |
| `CheckNIK(s)` | 16 digits, a region code, a serial other than `0000`, and a real birth date (`DDMMYY`, day + 40 for women). Returns the digits |
| `CheckNITKU(nitku, npwp16)` | 22 digits; with `npwp16`, must start with it. The last six digits are the place's sequence, `000000` the head office |
| `CheckPassport(s)` | 5–20 letters and digits, returned in capitals |
| `SuspectNPWP(npwp16)` | An old-format NPWP whose check digit (Luhn over its first nine digits, as python-stdnum checks it) does not match. DJP has not published the algorithm: show a warning, never refuse |
| `Mask(s)` | `3201********0001` — for lists and responses that need not show the whole number (UU PDP) |
| `Digits(s)` | Only the digits of `s` |

Every check returns `taxid.ErrFormat`; the caller turns it into its own field message.

## Changes

- **0.1.0** — first version: NPWP, NIK, NITKU, passport checks; the NPWP check digit as a
  warning; masking.
