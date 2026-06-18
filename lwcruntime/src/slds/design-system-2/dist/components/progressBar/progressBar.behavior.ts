import { computed } from '@fds-uif/behavior';
// doesnt seem like primitives are used in lightning

// <number> is used to specify the type of the computed value in a generic type way.
//  - computed<number> = "Hey computed, you're going to return a number"
//  - (props): number => = "This arrow function returns a number"
export const percentValue = computed<number>((props) => {
  const { value } = props;

  if (!value || value <= 0) {
    return 0;
  }
  if (value >= 100) {
    return 100;
  }
  return Math.round(value);
}, {
  description: 'Percentage value clamped between 0-100 and rounded to nearest integer.',
  // deps: ['value'] // Dependencies for optimization
});
// get percentValue(): number {
//   const { value } = this;

//   if (!value || value <= 0) {
//       return 0;
//   }
//   if (value >= 100) {
//       return 100;
//   }
//   return Math.round(value);
// }

export const computedStyle = computed<string>((props) => {
  return `width: ${props.percentValue}%;`;
}, {
  description: 'Computed style for the progress bar',
  //deps: ['percentValue']
});
// get computedStyle(): string {
//   return `width: ${this.percentValue}%;`;
// }

// straight from LBC
export interface NumberFormatOptions {
  style?: 'decimal' | 'currency' | 'percent';
  currency?: string;
  minimumIntegerDigits?: number;
  minimumFractionDigits?: number;
  maximumFractionDigits?: number;
  minimumSignificantDigits?: number;
  maximumSignificantDigits?: number;
}

// straight from LBC
export interface NumberFormatter {
  format(value: number): string;
}

const labelProgress = `Progress {0}`;
const labelProgressBar = `Progress Bar`;

// straight from LBC
export function numberFormat(options: NumberFormatOptions): NumberFormatter;

// ???
// Temporary workaround until we get real label support. New label entries must
// also be added to the static `labels` prop inside the class.
// https://git.soma.salesforce.com/raptor/raptor/issues/196
const i18n = {
  progress: labelProgress, // <- this is an import
  progressBar: labelProgressBar, // <- this is an import
};

export const assistiveText = computed<string>((props) => {
  const formattedPercent = numberFormat({ style: 'percent' }).format(
    props.percentValue / 100
  );

  // replace with: (one more utils) https://falcon.devhub.internal.salesforce.com/aihub/code-search/gitcore.soma.salesforce.com/core-2206/core-262-public@p4/262-main-e76ee021712b109b0e637ecbc2591514b86625f8/-/blob/core/ui-lightning-components/modules/lightning/utils/labelUtils.ts?q=repo%3Agitcore.soma.salesforce.com%2Fcore-2206%2Fcore-262-public+file%3Acore%2Fui-lightning-components%2Fmodules%2Flightning%2Futils%2F+context%3Asf_core+labelUtils+path%3Aui-lightning-components&patternType=regex
  // formatLabel(i18n.progress, formattedPercent);

  // if i didnt have utils i would do: but i should use utils above
  return `${i18n.progress.replace('{0}', formattedPercent)}`;
  // ! need spike 264 for localization- same system as core but make it work offcore

}, {
  description: 'Assistive text for the progress bar',
  // deps: ['percentValue']
});
// get assistiveText(): string {
//   const formattedPercent = numberFormat({ style: 'percent' }).format(
//       this.percentValue / 100
//   );

//   return `${i18n.progress} ${formattedPercent}`;
// }
