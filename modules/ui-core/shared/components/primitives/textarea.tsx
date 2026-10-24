import * as React from "react"
import { cn } from "cn"

import { FIELD } from "@/shared/lib/field-classes"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(FIELD, "h-auto min-h-[84px] py-2 field-sizing-content", className)}
      {...props}
    />
  )
}

export { Textarea }
