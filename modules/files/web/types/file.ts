export interface StoredFile {
  id: number;
  owner: string;
  name: string;
  content_type: string;
  size: number;
  sha256: string;
  created_at: string;
  /** Attached to a posted document: kept for good, cannot be deleted. */
  locked: boolean;
}
