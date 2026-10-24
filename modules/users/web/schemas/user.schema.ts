import { z } from "zod";

// Mirrors the API's rules (domain/user.go) so most mistakes show before a round trip.
const base = {
  name: z.string().trim().min(1, "Nama wajib diisi").max(120, "Nama terlalu panjang"),
  email: z.string().trim().min(1, "Email wajib diisi").email("Email tidak valid"),
  is_active: z.boolean(),
};

export const createUserSchema = z.object({
  ...base,
  password: z.string().min(8, "Minimal 8 karakter").max(72, "Maksimal 72 karakter"),
});

export const updateUserSchema = z.object({
  ...base,
  // Empty keeps the current password.
  password: z.union([z.literal(""), z.string().min(8, "Minimal 8 karakter").max(72, "Maksimal 72 karakter")]),
});

export type CreateUserValues = z.infer<typeof createUserSchema>;
export type UpdateUserValues = z.infer<typeof updateUserSchema>;
