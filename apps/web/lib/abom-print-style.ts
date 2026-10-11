/**
 * Stylesheet of the printable Agent Bill of Materials report that the supply
 * chain page writes into its own window. The report is a standalone document
 * printed on paper: it loads none of the dashboard's theme tokens, so its
 * colours are fixed here, outside the page's source, which takes every colour
 * from the theme.
 */
export const ABOM_PRINT_STYLE = `
  body { font-family: system-ui, sans-serif; padding: 40px; max-width: 1000px; margin: 0 auto; }
  h1 { color: #1e40af; border-bottom: 2px solid #3b82f6; padding-bottom: 10px; }
  h2 { color: #1e3a8a; margin-top: 30px; }
  h3 { color: #1e40af; margin-top: 20px; }
  .meta { color: #6b7280; font-size: 14px; margin-bottom: 30px; }
  .summary-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px; margin: 20px 0; }
  .summary-card { background: #f3f4f6; padding: 15px; border-radius: 8px; }
  .summary-card h4 { margin: 0 0 5px 0; color: #374151; font-size: 14px; }
  .summary-card p { margin: 0; font-size: 24px; font-weight: 600; color: #1e40af; }
  table { width: 100%; border-collapse: collapse; margin: 15px 0; }
  th { background: #e5e7eb; text-align: left; padding: 10px; font-size: 12px; text-transform: uppercase; }
  td { padding: 10px; border-bottom: 1px solid #e5e7eb; font-size: 14px; }
  td.url { font-size: 12px; color: #6b7280; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; margin: 2px; }
  .badge-green { background: #dcfce7; color: #166534; }
  .badge-blue { background: #dbeafe; color: #1e40af; }
  .badge-yellow { background: #fef3c7; color: #92400e; }
  .footer { margin-top: 50px; padding-top: 20px; border-top: 1px solid #e5e7eb; color: #6b7280; font-size: 12px; }
  @media print { body { padding: 20px; } }
`;
