export interface Permission {
  name: string;
  module: string;
  description: string;
}

export interface Role {
  id: number;
  name: string;
  description: string;
  is_system: boolean;
  permissions: string[];
  member_count: number;
  created_at: string;
  updated_at: string;
}

export interface RbacUser {
  id: number;
  name: string;
  email: string;
  is_active: boolean;
  /** The user directory no longer knows this user. */
  missing?: boolean;
}

/** Granted to the Administrator role: every permission, including future ones. */
export const WILDCARD = "*";
