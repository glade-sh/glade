Fresh 30-day Developer scratch org; deleted and deletion verified.
At each API version TransformCollection metadata deployment succeeded. Anonymous Apex at the matching API inserted a Contact with a non-null Title, mapped its Id and an unassigned input into a Contact, returned a collection, updated it, and read Title back.
At each API version both duplicate-target metadata deployments failed separately with the recorded error. The first maps null before non-null; the second maps non-null before null. Neither flow executed.
