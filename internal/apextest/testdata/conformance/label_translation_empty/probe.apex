for (String labelName : new List<String>{'EmptyLabel','MissingLabel','FilledLabel'}) {
 System.debug('CAPTURE|' + labelName + '|' + System.Label.get('', labelName, 'fr') + '|' + System.Label.translationExists('', labelName, 'fr'));
}
