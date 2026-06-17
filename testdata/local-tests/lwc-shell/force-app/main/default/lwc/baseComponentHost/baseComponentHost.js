import { LightningElement } from 'lwc';

export default class BaseComponentHost extends LightningElement {
    columns = [{ label: 'Name', fieldName: 'name' }];
    rows = [{ id: '1', name: 'VF Local Account' }];
    options = [
        { label: 'Alpha', value: 'alpha' },
        { label: 'Beta', value: 'beta' }
    ];
    selectedOptions = ['alpha'];
    pills = [
        { type: 'icon', label: 'Package', name: 'package' },
        { type: 'icon', label: 'Phase 1', name: 'phase1' }
    ];
    treeItems = [
        {
            label: 'Package',
            name: 'package',
            expanded: true,
            items: [{ label: 'Phase 1', name: 'phase1' }]
        }
    ];
    phase3Options = [
        { label: 'Alpha', value: 'alpha' },
        { label: 'Beta', value: 'beta' },
        { label: 'Gamma', value: 'gamma' }
    ];
    phase3Values = ['alpha', 'beta'];
    richText = '<p>Phase 3 rich text</p>';
    treeGridColumns = [{ label: 'Name', fieldName: 'name' }];
    treeGridRows = [
        {
            id: '1',
            name: 'Root Provider',
            _children: [{ id: '2', name: 'Child Provider' }]
        }
    ];
    mapMarkers = [
        {
            title: 'Twin Lakes',
            location: { City: 'Twin Lakes', State: 'AK' }
        }
    ];
    carouselImage = 'data:image/gif;base64,R0lGODlhAQABAAAAACw=';
}
