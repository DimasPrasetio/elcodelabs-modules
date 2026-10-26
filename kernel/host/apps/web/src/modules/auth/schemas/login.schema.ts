import { z } from "zod";

export const loginSchema = z.object({
  email: z.string().email("Email tidak valid"),
  password: z.string().min(8, "Kata sandi minimal 8 karakter"),
});

export type LoginFormValues = z.infer<typeof loginSchema>;
