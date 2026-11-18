@IsTest private class CaptureLabel67 {
 @IsTest static void french() {
  System.assertEquals('Master empty', System.Label.get('', 'EmptyLabel', 'fr'));
  System.assertEquals('Master missing', System.Label.get('', 'MissingLabel', 'fr'));
  System.assertEquals('Traduit', System.Label.get('', 'FilledLabel', 'fr'));
  System.assertEquals(false, System.Label.translationExists('', 'EmptyLabel', 'fr'));
  System.assertEquals(false, System.Label.translationExists('', 'MissingLabel', 'fr'));
  System.assertEquals(true, System.Label.translationExists('', 'FilledLabel', 'fr'));
 }
 @IsTest static void defaults() {
  System.assertEquals('Master empty', Label.EmptyLabel);
  System.assertEquals('Master missing', Label.MissingLabel);
  System.assertEquals('Master filled', Label.FilledLabel);
 }
}
