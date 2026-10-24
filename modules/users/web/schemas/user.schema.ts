import { z } from "zod";

// Mirrors the API's rules (domain/user.go) so most mistakes show before a round trip.
const base = {
  name: z.string().trim().min(1, "Name is required").max(120, "Name is too long"),
  email: z.string().trim().min(1, "Email is required").email("Enter a valid email"),
  is_active: z.boolean(),
};

export const createUserSchema = z.object({
  ...base,
  password: z.string().min(8, "At least 8 characters").max(72, "At most 72 characters"),
});

export const updateUserSchema = z.object({
  ...base,
  // Empty keeps the current password.
  password: z.union([z.literal(""), z.string().min(8, "At least 8 characters").max(72, "At most 72 characters")]),
});

export type CreateUserValues = z.infer<typeof createUserSchema>;
export type UpdateUserValues = z.infer<typeof updateUserSchema>;
